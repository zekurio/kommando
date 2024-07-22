package kommando

import "github.com/bwmarrin/discordgo"

// Context holds our data from the interaction neatly in one object
type Context interface {
	Responder

	Channel() (channel *discordgo.Channel, err error)

	Guild() (guild *discordgo.Guild, err error)

	User() (user *discordgo.User, err error)

	Options() []*discordgo.ApplicationCommandInteractionDataOption

	Command() Command

	SlashCommand() SlashCommand

	// TODO work on other command types
}

// Responder handles responding and follow up messages
type Responder interface {
	Respond(r *discordgo.InteractionResponse) (err error)

	RespondMessage(content string) (err error)

	RespondEmbed(embed *discordgo.MessageEmbed) (err error)

	RespondError(content, title string) (err error)

	GetSession() *discordgo.Session

	GetEvent() *discordgo.InteractionCreate

	GetKommando() *Kommando

	GetEphemeral() bool

	SetEphemeral(v bool)
}

type Ctx struct {
	responder

	cmd Command
}

var _ Context = (*Ctx)(nil)

func (c *Ctx) Channel() (channel *discordgo.Channel, err error) {
	return c.kommando.options.State.Channel(c.session, c.event.ChannelID)
}

func (c *Ctx) Guild() (guild *discordgo.Guild, err error) {
	return c.kommando.options.State.Guild(c.session, c.event.GuildID)
}

func (c *Ctx) User() (user *discordgo.User, err error) {
	return c.kommando.options.State.User(c.session, c.event.User.ID)
}

func (c *Ctx) Options() []*discordgo.ApplicationCommandInteractionDataOption {
	return c.event.ApplicationCommandData().Options
}

func (c *Ctx) Command() Command {
	return c.cmd
}

func (c *Ctx) SlashCommand() SlashCommand {
	return c.cmd.(SlashCommand)
}

type responder struct {
	responded bool
	kommando  *Kommando
	session   *discordgo.Session
	event     *discordgo.InteractionCreate
	ephemeral bool
}

var _ Responder = (*responder)(nil)

func (r *responder) messageFlags(p discordgo.MessageFlags) (f discordgo.MessageFlags) {
	f = p
	if r.ephemeral {
		f |= discordgo.MessageFlagsEphemeral
	}
	return
}

func (r *responder) Respond(res *discordgo.InteractionResponse) (err error) {
	if res.Data == nil {
		res.Data = new(discordgo.InteractionResponseData)
	}

	res.Data.Flags = r.messageFlags(res.Data.Flags)

	if r.responded {
		if res == nil || res.Data == nil {
			return
		}
		_, err = r.session.InteractionResponseEdit(r.event.Interaction, &discordgo.WebhookEdit{
			Content:         &res.Data.Content,
			Embeds:          &res.Data.Embeds,
			Components:      &res.Data.Components,
			Files:           res.Data.Files,
			AllowedMentions: res.Data.AllowedMentions,
		})
		if err != nil {
			return
		}
	} else {
		err = r.session.InteractionRespond(r.event.Interaction, res)
		r.responded = err == nil
	}

	return
}

func (r *responder) RespondMessage(content string) (err error) {
	return r.Respond(&discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

func (r *responder) RespondEmbed(embed *discordgo.MessageEmbed) (err error) {
	return r.Respond(&discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func (r *responder) RespondError(content, title string) (err error) {
	return r.RespondEmbed(&discordgo.MessageEmbed{
		Title:       title,
		Description: content,
		Color:       r.kommando.options.EmbedColors.Error,
	})
}

func (r *responder) GetSession() *discordgo.Session {
	return r.session
}

func (r *responder) GetEvent() *discordgo.InteractionCreate {
	return r.event
}

func (r *responder) GetKommando() *Kommando {
	return r.kommando
}

func (r *responder) GetEphemeral() bool {
	return r.ephemeral
}

func (r *responder) SetEphemeral(v bool) {
	r.ephemeral = v
}

// TODO handling sub commands
// https://discord.com/developers/docs/interactions/application-commands#subcommands-and-subcommand-groups

type SubCommandContext interface {
	Context

	GetSubCommandName() string
}

type subCommandCtx struct {
	Context

	subCommandName string
}

func (c *subCommandCtx) GetSubCommandName() string {
	return c.subCommandName
}

func (c *subCommandCtx) Options() []*discordgo.ApplicationCommandOption {
	var results []*discordgo.ApplicationCommandOption

	for _, o := range c.Options() {
		if o.Name == c.subCommandName {
			results = append(results, o)
		}
	}

	return results
}

func (c *subCommandCtx) HandleSubCommands(handler []CommandHandler) (err error) {
	return handleSubCommands(c, handler)
}

func handleSubCommands(c *subCommandCtx, handler []CommandHandler) (err error) {
	options := c.Options()[0]
	for _, h := range handler {
		if options.Type != h.Type() || options.Name != h.OptionName() {
			continue
		}

		// TODO handle subcommands context in kommando
		break
	}

	return err
}

//var _ SubCommandContext = (*subCommandCtx)(nil)

type CommandHandler interface {
	Type() discordgo.ApplicationCommandOptionType
	OptionName() string
	RunHandler(ctx SubCommandContext) error
}

type SubCommandHandler struct {
	Name string // Name is defined by the options
	Run  func(ctx SubCommandContext) error
}
