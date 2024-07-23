package kommando

import (
	"errors"
	"log"
	"regexp"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/zekurio/kommando/state"
)

type Kommando struct {
	s *discordgo.Session

	cmds       map[string]Command
	idCache    map[string]string
	lockCmds   sync.RWMutex
	ctxPool    sync.Pool
	subCtxPool sync.Pool

	options *Options
}

type EmbedColors struct {
	Default int

	Error int
}

type Options struct {
	State state.State

	EmbedColors EmbedColors

	OnSystemError  func(err error)
	OnCommandError func(ctx Context, err error)
}

var defaultOptions = Options{
	State: state.NewSessionWrapped(),

	EmbedColors: EmbedColors{
		Default: 0xfe640b,
		Error:   0xd20f39,
	},

	OnSystemError: func(err error) {
		log.Printf("kommando - system error: %s", err)
	},

	OnCommandError: func(ctx Context, err error) {
		log.Printf("kommando - error in command: %s, err: %s", ctx.Command().Name(), err)
	},
}

func New(s *discordgo.Session, options ...Options) (h *Kommando, err error) {
	h = &Kommando{
		cmds:    make(map[string]Command),
		idCache: make(map[string]string),
		s:       s,
		ctxPool: sync.Pool{
			New: func() interface{} {
				return &Ctx{}
			},
		},
		subCtxPool: sync.Pool{
			New: func() interface{} { return &subCommandCtx{} },
		},
	}

	h.options = &defaultOptions

	if len(options) > 0 {
		o := options[0]

		if o.State != nil {
			h.options.State = o.State
		}

		if o.OnSystemError != nil {
			h.options.OnSystemError = o.OnSystemError
		}

		if o.OnCommandError != nil {
			h.options.OnCommandError = o.OnCommandError
		}
	}

	s.AddHandler(h.onReady)
	s.AddHandler(h.onInteractionCreate)

	return
}

func (c *Kommando) RegisterCommands(cmds ...Command) (err error) {
	c.lockCmds.Lock()
	defer c.lockCmds.Unlock()

	regex, _ := regexp.Compile(`^[\-_0-9\p{L}\p{Devanagari}\p{Thai}]{1,32}$`)

	for _, cmd := range cmds {
		if cmd.Name() == "" {
			err = errors.New("command name cannot be empty")
			return
		}

		res := regex.MatchString(cmd.Name())

		if err != nil || !res {
			return errors.New("command name doesn't parse regex")
		}

		c.cmds[cmd.Name()] = cmd
	}

	return
}

func (c *Kommando) UnregisterCommands() {
	self, err := c.options.State.SelfUser(c.s)
	if err != nil {
		return
	}

	for name, id := range c.idCache {
		err = c.s.ApplicationCommandDelete(self.ID, "", id)
		if err != nil {
			c.options.OnSystemError(err)
		}
		delete(c.idCache, name)
	}
}

func (c *Kommando) onReady(s *discordgo.Session, e *discordgo.Ready) {
	var (
		cachedCommand *discordgo.ApplicationCommand
		err           error
		update        []*discordgo.ApplicationCommand
	)

	for name, cmd := range c.cmds {
		guildId := "" // TODO handle guild scoped commands
		if _, ok := c.idCache[name]; ok {
			appCommand := toApplicationCommand(cmd)
			update = append(update, appCommand)
		} else {
			cachedCommand, err = s.ApplicationCommandCreate(e.User.ID, guildId, toApplicationCommand(cmd))
			if err != nil {
				c.options.OnSystemError(err)
			} else {
				c.idCache[name] = cachedCommand.ID
			}
		}
	}

	if len(update) > 0 {
		_, err = s.ApplicationCommandBulkOverwrite(e.User.ID, "", update)
		if err != nil {
			c.options.OnSystemError(err)
		}
	}
}

func (c *Kommando) onInteractionCreate(s *discordgo.Session, e *discordgo.InteractionCreate) {
	switch e.Type {
	case discordgo.InteractionApplicationCommand:
		c.appCommandInteraction(s, e)
	default:
		return
	}
}

func (c *Kommando) appCommandInteraction(s *discordgo.Session, e *discordgo.InteractionCreate) {
	c.lockCmds.RLock()
	cmd := c.cmds[e.ApplicationCommandData().Name]
	c.lockCmds.RUnlock()

	if cmd == nil {
		return
	}

	ch, err := c.options.State.Channel(s, e.Interaction.ChannelID)
	if err != nil {
		c.options.OnSystemError(err)
	}

	ctx := c.ctxPool.Get().(*Ctx)
	defer c.ctxPool.Put(ctx)

	ctx.responded = false
	ctx.kommando = c
	ctx.session = s
	ctx.event = e
	ctx.cmd = cmd
	ctx.ephemeral = false

	if ch.Type == discordgo.ChannelTypeDM || ch.Type == discordgo.ChannelTypeGroupDM {
		if goCmd, ok := cmd.(GuildOnly); ok || goCmd.GuildOnly() {
			c.options.OnCommandError(ctx, errors.New("command only available in guild"))
			return
		}
	}

	err = cmd.Exec(ctx)
	if err != nil {
		c.options.OnSystemError(err)
	}
}
