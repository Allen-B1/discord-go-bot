package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"os/exec"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

var packages = [][3]string{
	{"archive/tar", "NewReader"},
	{"archive/zip", "NewReader"},
	{"bufio", "NewReader"},
	{"bytes", "Contains"},
	{"compress/bzip2", "NewReader"},
	{"compress/flate", "NewReader"},
	{"compress/gzip", "NewReader"},
	{"compress/lzw", "NewReader"},
	{"compress/zlib", "NewReader"},
	{"container/heap", "Init"},
	{"container/ring", "New"},
	{"container/list", "New"},
	{"context", "WithCancel"},
	{"crypto/aes", "NewCipher"},
	{"crypto/cipher", "NewGCM"},
	{"crypto/des", "NewCipher"},
	{"crypto/dsa", "Verify"},
	{"crypto/ecdsa", "Verify"},
	{"crypto/ed25519", "Verify"},
	{"crypto/elliptic", "GenerateKey"},
	{"crypto/hmac", "New"},
	{"crypto/md5", "New"},
	{"crypto/rand", "Int", "crand"},
	{"crypto/rc4", "NewCipher"},
	{"crypto/rsa", "GenerateKey"},
	{"crypto/sha1", "New"},
	{"crypto/sha256", "New"},
	{"crypto/sha512", "New"},
	{"crypto/subtle", "ConstantTimeSelect"},
	{"crypto/tls", "NewListener"},
	{"crypto/x509", "NewCertPool"},
	{"encoding/ascii85", "NewEncoder"},
	{"encoding/asn1", "Marshal"},
	{"encoding/base32", "NewEncoder"},
	{"encoding/base64", "NewEncoder"},
	{"encoding/binary", "Size"},
	{"encoding/csv", "NewReader"},
	{"encoding/gob", "NewEncoder"},
	{"encoding/hex", "Encode"},
	{"encoding/json", "Marshal"},
	{"encoding/pem", "Encode"},
	{"encoding/xml", "Marshal"},
	{"errors", "New"},
	{"expvar", "Do"},
	{"flag", "Int"},
	{"fmt", "Println"},
	{"os", "Exit"},
	{"math/big", "NewInt"},
	{"math/rand", "Int"},
	{"time", "Now"},
	{"strings", "Contains"},
}

var TOKEN = os.Getenv("TOKEN")

func parseMessage(msg string) string {
	type_ := 3
	i := strings.Index(msg, "```")
	j := -1
	if i == -1 {
		i = strings.Index(msg, "`")
		type_ = 1
		if i == -1 {
			return ""
		}

		j = strings.Index(msg[i+1:], "`")
		if j < 0 {
			return ""
		}
		j += i + 1
	} else {
		j = strings.Index(msg[i+1:], "```")
		if j < 0 {
			return ""
		}
		j += i + 1
	}

	rawCode := ""
	if type_ == 3 {
		k := strings.Index(msg[i+1:], "\n")
		if k == -1 {
			return ""
		}
		k += i + 1
		rawCode = msg[k+1 : j]
	} else {
		rawCode = msg[i+1 : j]
	}

	imports := ""
	uses := ""
	for _, pkg := range packages {
		if pkg[2] != "" {
			imports += pkg[2] + " \"" + pkg[0] + "\";"
			uses += "_ = " + pkg[2] + "." + pkg[1] + ";"
		} else {
			imports += "\"" + pkg[0] + "\";"
			uses += "_ = " + path.Base(pkg[0]) + "." + pkg[1] + ";"
		}
	}

	if type_ == 1 {
		return fmt.Sprintf(`package main

import (
	%s
)

func main() {
	%s
	rand.Seed(time.Now().UnixNano())
	fmt.Println(%s)
}`, imports, uses, rawCode)
	}

	code := rawCode
	if strings.Index(rawCode, "func main") == -1 {
		code = fmt.Sprintf(`package main

import (
	%s
)

func main() {
	%s
	rand.Seed(time.Now().UnixNano())
	%s
}`, imports, uses, rawCode)
	}
	if strings.Index(code, "package") == -1 {
		code = "package main\n" + code
	}

	return code
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
		Description: "" +
			"!go `1 + 3 * rand.Intn(10)`\n\n" +
			"!go ```go\n" +
			"fmt.Println(\"code here\")\n" +
			"os.Exit(1)\n" +
			"```",
	})
}

func handleRunCommand(dg *discordgo.Session, m *discordgo.Message) {
	// handle run command
	code := parseMessage(m.Content)
	if code == "" {
		return
	}

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
			log.Println("unknown error", err)
		}
		return
	}
	texts := make(map[string]string)
	for _, item := range o.output {
		texts[item[0]] += item[1]
	}

	ellipses := make(map[string]string)
	for key, _ := range texts {
		texts[key] = strings.Replace(texts[key], "```", "`\u200b`\u200b`", -1)
		if len(texts[key]) > 1000 {
			texts[key] = texts[key][:1000]
			ellipses[key] = "*truncated*"
		}
	}

	color := 0xDF9F1F
	if o.code == 0 {
		color = 0x5FDF1F
	}

	fields := []*discordgo.MessageEmbedField{}
	if texts["stdout"] != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Stdout", Value: "```\n" + texts["stdout"] + "```" + ellipses["stdout"], Inline: false})
	}
	if texts["stderr"] != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Stderr", Value: "```\n" + texts["stderr"] + "```" + ellipses["stderr"], Inline: texts["stdout"] != ""})
	}

	if o.code != 0 || (texts["stderr"] == "" && texts["stdout"] == "") {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Exit Code", Value: fmt.Sprint(o.code), Inline: true})
	}

	_, err = dg.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Fields: fields,
			Color:  color,
		},
		Reference: m.Reference(),
	})
	if err != nil {
		log.Println(err)
	}
}

func main() {
	dg, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	log.Println("Bot now running")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}
