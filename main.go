package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"os/exec"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

var logger *log.Logger

var TOKEN = os.Getenv("TOKEN")

func parseMessage(msg string) (string, string) {
	type_ := 3
	i := strings.Index(msg, "```")
	j := -1
	if i == -1 {
		i = strings.Index(msg, "`")
		type_ = 1
		if i == -1 {
			return "", ""
		}

		j = strings.Index(msg[i+1:], "`")
		if j < 0 {
			return "", ""
		}
		j += i + 1
	} else {
		j = strings.Index(msg[i+1:], "```")
		if j < 0 {
			return "", ""
		}
		j += i + 1
	}

	rawCode := ""
	if type_ == 3 {
		k := strings.Index(msg[i+1:], "\n")
		if k == -1 {
			return "", ""
		}
		k += i + 1
		rawCode = msg[k+1 : j]
	} else {
		rawCode = msg[i+1 : j]
	}

	return codeFilter(rawCode, type_), rawCode
}

func hasMention(mentions []*discordgo.User, user *discordgo.User) bool {
	for _, mention := range mentions {
		if mention.ID == user.ID {
			return true
		}
	}
	return false
}

func handleHelpCommand(dg *discordgo.Session, m *discordgo.Message) {
	dg.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title: "!go usage",
		Color: 0x1ED6D6,
		Description: "" +
			"!go `1 + 3 * rand.Intn(10)`\n\n" +
			"!go ```go\n" +
			"fmt.Println(\"code here\")\n" +
			"os.Exit(1)\n" +
			"```",
	})
}

func getEmoji(dg *discordgo.Session, guildId string, emojiName string) string {
	emojis, err := dg.GuildEmojis(guildId)
	if err != nil {
		log.Println(err)
		return ""
	}

	for _, emoji := range emojis {
		if emoji.Name == emojiName {
			return emoji.APIName()
		}
	}
	return ""
}

func handleRunCommand(dg *discordgo.Session, m *discordgo.Message) {
	// handle run command
	code, rawCode := parseMessage(m.Content)
	if code == "" {
		i := strings.Index(m.Content, "http://")
		if i == -1 {
			i = strings.Index(m.Content, "https://")
		}
		if i == -1 {
			return
		}
		resp, err := http.Get(strings.TrimSpace(m.Content[i:]))
		if err != nil {
			logger.Println("msg " + m.Author.Username + ": " + "web fail " + m.Content[i:])
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Println("msg " + m.Author.Username + ": " + "web fail " + m.Content[i:])
		}
		logger.Println("msg " + m.Author.Username + ": " + "web " + m.Content[i:])
		rawCode = string(body)
		code = codeFilter(rawCode, 3)
	}

	if !strings.Contains(m.Content, "```") {
		logger.Println("msg " + m.Author.Username + ": " + rawCode)
	} else {
		logger.Println("msg " + m.Author.Username + ":\n" + rawCode + "\n")
	}

	// add reaction
	err := dg.MessageReactionAdd(m.ChannelID, m.ID, getEmoji(dg, m.GuildID, "gopher"))
	if err != nil {
		logger.Println("error " + err.Error())
	}

	// set typing indicator
	stop := make(chan bool)
	go func() {
		for {
			dg.ChannelTyping(m.ChannelID)
			select {
			case <-stop:
				return
			case <-time.After(300 * time.Millisecond):
			}
		}
	}()
	o, err := runCode(code)
	stop <- true
	if err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			dg.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Embed: &discordgo.MessageEmbed{
					Title:       "Compilation Error",
					Description: "```\n" + string(eerr.Stderr) + "```",
					Color:       0x851313,
				},
				Reference: m.Reference(),
			})
		} else {
			logger.Println("unknown error", err)
		}
		return
	}

	// combine stdout/stderr
	texts := make(map[string]string)
	for _, item := range o.output {
		texts[item[0]] += item[1]
	}

	// truncate stdout/stderr
	ellipses := make(map[string]string)
	for key, _ := range texts {
		texts[key] = strings.Replace(texts[key], "```", "`\u200b`\u200b`", -1)
		if len(texts[key]) > 1000 {
			texts[key] = texts[key][:1000]
			ellipses[key] = "*truncated*"
		}
	}

	getLast := func(arr []string) string {
		if len(arr) == 0 {
			return ""
		}
		return arr[len(arr)-1]
	}

	color := 0x1ED6D6
	if colorStr := getLast(o.info["embed_color"]); colorStr != "" {
		color64, _ := strconv.ParseInt(colorStr, 16, 64)
		color = int(color64)
	}

	desc := getLast(o.info["embed_description"])
	title := getLast(o.info["embed_title"])
	titleUrl := getLast(o.info["embed_url"])
	timestamp := getLast(o.info["embed_timestamp"])

	footer := (*discordgo.MessageEmbedFooter)(nil)
	if footerFields := strings.Split(getLast(o.info["embed_footer"]), "\u200b"); len(footerFields) > 1 {
		footerText, footerIcon := footerFields[0], footerFields[1]
		footer = &discordgo.MessageEmbedFooter{
			Text:    footerText,
			IconURL: footerIcon,
		}
	}

	image := (*discordgo.MessageEmbedImage)(nil)
	if imageFields := strings.Split(getLast(o.info["embed_image"]), "\u200b"); len(imageFields) > 2 {
		width, _ := strconv.Atoi(imageFields[1])
		height, _ := strconv.Atoi(imageFields[2])
		image = &discordgo.MessageEmbedImage{URL: imageFields[0], Width: width, Height: height}
	}
	thumbnail := (*discordgo.MessageEmbedThumbnail)(nil)
	if imageFields := strings.Split(getLast(o.info["embed_thumbnail"]), "\u200b"); len(imageFields) > 2 {
		width, _ := strconv.Atoi(imageFields[1])
		height, _ := strconv.Atoi(imageFields[2])
		thumbnail = &discordgo.MessageEmbedThumbnail{URL: imageFields[0], Width: width, Height: height}
	}
	video := (*discordgo.MessageEmbedVideo)(nil)
	if imageFields := strings.Split(getLast(o.info["embed_video"]), "\u200b"); len(imageFields) > 2 {
		width, _ := strconv.Atoi(imageFields[1])
		height, _ := strconv.Atoi(imageFields[2])
		video = &discordgo.MessageEmbedVideo{URL: imageFields[0], Width: width, Height: height}
	}

	provider := (*discordgo.MessageEmbedProvider)(nil)
	if providerFields := strings.Split(getLast(o.info["embed_provider"]), "\u200b"); len(providerFields) > 1 {
		provider = &discordgo.MessageEmbedProvider{
			Name: providerFields[0],
			URL:  providerFields[1],
		}
	}

	author := (*discordgo.MessageEmbedAuthor)(nil)
	if authorFields := strings.Split(getLast(o.info["embed_author"]), "\u200b"); len(authorFields) > 2 {
		author = &discordgo.MessageEmbedAuthor{
			Name:    authorFields[0],
			URL:     authorFields[1],
			IconURL: authorFields[2],
		}
	}

	fields := []*discordgo.MessageEmbedField{}
	if texts["stdout"] != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Stdout", Value: "```\n" + texts["stdout"] + "```" + ellipses["stdout"], Inline: false})
	}
	if texts["stderr"] != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Stderr", Value: "```\n" + texts["stderr"] + "```" + ellipses["stderr"], Inline: texts["stdout"] != ""})
	}

	for _, field := range o.info["embed_field"] {
		splits := strings.Split(field, "\u200b")
		title := splits[0]
		inline := splits[1] == "true"
		text := strings.Join(splits[2:], "\u200b")
		fields = append(fields, &discordgo.MessageEmbedField{Name: title, Inline: inline, Value: text})
	}

	if o.code != 0 || (texts["stderr"] == "" && texts["stdout"] == "" && desc == "") {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Exit Code", Value: fmt.Sprint(o.code), Inline: true})
	}

	_, err = dg.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Fields:      fields,
			Description: desc,
			Title:       title,
			URL:         titleUrl,
			Color:       color,
			Timestamp:   timestamp,
			Footer:      footer,
			Image:       image, Thumbnail: thumbnail, Video: video,
			Provider: provider, Author: author,
		},
		Reference: m.Reference(),
	})
	if err != nil {
		logger.Println(err)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	logfile, err := os.OpenFile("logs/"+fmt.Sprint(time.Now().Unix())+".txt", os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer logfile.Close()
	logger = log.New(io.MultiWriter(os.Stdout, logfile), "", log.LstdFlags)
	dg, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		logger.Fatal(err)
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID != s.State.User.ID {
			if strings.HasPrefix(m.Content, "!go") || hasMention(m.Mentions, dg.State.User) {
				if strings.HasPrefix(m.Content, "!gohelp") || (hasMention(m.Mentions, dg.State.User) && !strings.Contains(m.Content, "`")) {
					// handle help command
					handleHelpCommand(dg, m.Message)
				}

				handleRunCommand(dg, m.Message)
			}
		}
	})

	err = dg.Open()
	if err != nil {
		logger.Fatal(err)
	}

	logger.Println(dg.State.User.Username + "#" + dg.State.User.Discriminator + " online")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}
