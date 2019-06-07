package main

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DiscordTokenEnvKey = "DISCORD_TOKEN"
)

var (
	DiscordToken string
	Connections  map[string]*Connection
)

func init() {
	token, exists := os.LookupEnv(DiscordTokenEnvKey)
	if !exists {
		panic(fmt.Errorf("failed to find discord token at env %q", DiscordTokenEnvKey))
	}
	DiscordToken = token

	Connections = make(map[string]*Connection)
}

type Connection struct {
	Voice     *discordgo.VoiceConnection
	GuildName string
}

func main() {
	DetectionMain()

	//logger := log.New()
	//logger.Info("logging in...")
	//discord, err := discordgo.New("Bot " + DiscordToken)
	//if err != nil {
	//	panic(err)
	//}
	//discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) { messageCreate(logger, s, m) })
	//err = discord.Open()
	//if err != nil {
	//	panic(err)
	//}
	//logger.Info("logged in success!")
	//sc := make(chan os.Signal, 1)
	//signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	//<-sc
	//logger.Info("shutting down...")
	//err = discord.Close()
	//if err != nil {
	//	panic(err)
	//}
}

func findVoiceState(id string, states []*discordgo.VoiceState) (*discordgo.VoiceState, error) {
	for _, state := range states {
		if state.UserID == id {
			return state, nil
		}
	}
	return nil, errors.New("could not find voice state for user")
}

func Listen(logger *log.Logger, connection *Connection) {
	var noPacket bool
	for {
		select {
		case packet := <- connection.Voice.OpusRecv:
			noPacket = false
			epoch, err := strconv.ParseInt(string(packet.Timestamp), 10, 64)
			if err != nil {
				logger.Error(err)
				continue
			}
			timestamp := time.Unix(epoch, 0)
			formattedTime := timestamp.Format(time.RFC822)
			logger.WithField("guild", connection.GuildName).Infof("%s, got %d bytes!", formattedTime, len(packet.Opus))
		default:
			if !noPacket {
				logger.Info("no packets :(")
				noPacket = true
			}
		}
	}
}

func messageCreate(logger *log.Logger, s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	sentTime, err := m.Timestamp.Parse()
	if err != nil {
		logger.Error(err)
		return
	}
	formattedTime := sentTime.Format(time.RFC822)
	guild, err := s.Guild(m.GuildID)
	if err != nil {
		logger.Error(err)
		return
	}
	var content string
	if strings.TrimSpace(m.Content) == "" {
		if len(m.Attachments) > 0 {
			content = "<file>"
		} else {
			content = "<blank>"
		}
	} else {
		content = m.Content
	}
	logger.WithField("guild", guild.Name).Infof("%s %s: %s", formattedTime, m.Author.Username, content)
	selfMention := s.State.User.Mention()
	if strings.Contains(m.Content, selfMention) {
		runeMessage := []rune(m.Content)
		mentionLength := len([]rune(selfMention)) + 1
		message := string(runeMessage[mentionLength:])
		if message == "summon" {
			authorID := m.Author.ID
			voiceChannel, err := findVoiceState(authorID, guild.VoiceStates)
			if err != nil {
				logger.Error(err)
				return
			}
			voiceState, err := s.ChannelVoiceJoin(guild.ID, voiceChannel.ChannelID, false, false)
			if err != nil {
				logger.Error(err)
				return
			}
			connection := Connection{
				Voice:     voiceState,
				GuildName: guild.Name,
			}
			Connections[guild.ID] = &connection
			totalConnections := len(Connections)
			go Listen(logger, &connection)
			logger.Infof("Added %q (%s) to connections list (%d total)", guild.ID, guild.Name, totalConnections)
		} else if message == "leave" {
			state := Connections[guild.ID]
			err := state.Voice.Disconnect()
			if err != nil {
				logger.Error(err)
				return
			}
		}
	}
}
