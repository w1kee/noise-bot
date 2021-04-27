package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

const (
	tokFile = "token"
	prefix  = "!!"
	// 256 is max
	volume = 127
)

type PlayRequest struct {
	soundPath string
	vs        *discordgo.VoiceState
}

//go:embed sounds
var soundsFS embed.FS
var soundMap = make(map[string]string)
var soundList = make([]string, 0)
var reqChan = make(chan PlayRequest)

var cmdRegexp = regexp.MustCompile("^[A-Za-z0-9]+$")

var encodeOptions = dca.StdEncodeOptions

func main() {
	encodeOptions.Volume = volume
	setupSounds()
	tok, ok := getToken("TOKEN")
	if !ok {
		log.Fatal("TOKEN envvar not specified")
	}
	s, err := discordgo.New("Bot " + tok)
	if err != nil {
		log.Fatal("error creating session:", err)
	}

	s.AddHandler(handleMessage)

	go player(s)

	if err = s.Open(); err != nil {
		log.Fatal("error opening session:", err)
	}

	err = s.UpdateListeningStatus(prefix + "help")
	if err != nil {
		log.Print("can't set status:", err)
	}

	<-make(chan struct{})
}

func getToken(key string) (string, bool) {
	return os.LookupEnv(key)
}

func player(session *discordgo.Session) {
	var (
		vc   *discordgo.VoiceConnection
		es   *dca.EncodeSession
		dec  *dca.Decoder
		file fs.File
		err  error
		req  PlayRequest
	)
	cleanup := func() {
		es.Cleanup()
		file.Close()
		vc.Disconnect()
	}
	for {
		req = <-reqChan
		file, err = soundsFS.Open(req.soundPath)
		// remember to close file
		if err != nil {
			log.Print("setupSounds crapped out:", err)
			continue
		}
		vc, err = session.ChannelVoiceJoin(req.vs.GuildID, req.vs.ChannelID, false, true)
		if err != nil {
			log.Print("error joining voice channel:", err)
			continue
		}
		es, err = dca.EncodeMem(file, encodeOptions)
		// remember to cleanup
		if err != nil {
			log.Print("error creating encode session:", err)
		}

		dec = dca.NewDecoder(es)

		for {
			frame, err := dec.OpusFrame()
			if err != nil {
				if err != io.EOF {
					fmt.Print("error getting opus frame:", err)
				}

				break
			}

			select {
			case vc.OpusSend <- frame:
			case <-time.After(time.Second):
				log.Print("connection seems to have crapped out stopping this sound")
				break
			}
		}
		cleanup()
	}
}

func setupSounds() {
	fs.WalkDir(soundsFS, "sounds", func(filepath string, d fs.DirEntry, err error) error {
		// TODO: learn how fs.WalkDirFunc works with the err parameter, for now returning nil
		if err != nil {
			log.Print("error in WalkDirFunc:", err)
			return nil
		}
		// ignore file if it is a directory
		if d.IsDir() {
			return nil
		}

		ext := path.Ext(filepath)
		mt := mime.TypeByExtension(ext)
		// TODO: i don't like this, find a better way
		if !strings.HasPrefix(mt, "audio/") {
			log.Printf("ignoring %s, %s is not an audio mimetype", filepath, mt)
			return nil
		}

		extless := d.Name()[:len(d.Name())-len(ext)]
		// TODO: check if extless is a valid command /[A-Za-z0-9]+/
		if !cmdRegexp.Match([]byte(extless)) {
			log.Printf("ignoring %s, %s doesn't match the /%s/ regexp", filepath, extless, cmdRegexp)
			return nil
		}
		soundMap[prefix+extless] = filepath
		soundList = append(soundList, extless)
		return nil
	})
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Content == prefix+"help" {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:       "List of sounds",
			Color:       0x2ab7ca,
			Description: "`" + strings.Join(soundList, "` `") + "`",
			Footer: &discordgo.MessageEmbedFooter{
				Text: "Send `!!<sound>` to chat to play the sound in your current voice channel.",
			},
		})
		return
	}

	path, ok := soundMap[m.Content]
	if !ok {
		return
	}

	vs := findUserVoiceState(s, m.Author.ID, m.GuildID)
	if vs == nil {
		return
	}

	// sends only if a receiver is waiting on the other end
	select {
	case reqChan <- PlayRequest{path, vs}:
	default:
	}
}

func findUserVoiceState(session *discordgo.Session, userid, guildid string) *discordgo.VoiceState {
	for _, guild := range session.State.Guilds {
		for _, vs := range guild.VoiceStates {
			if vs.UserID == userid {
				return vs
			}
		}
	}
	return nil
}
