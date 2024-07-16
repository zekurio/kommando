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

func (c *responder) messageFlags(p discordgo.MessageFlags) (f discordgo.MessageFlags) {
	f = p
	if c.ephemeral {
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
