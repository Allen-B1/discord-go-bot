package main

import (
	"fmt"
	"path"
	"strings"
)

const API = `
type gobot_ struct{}

func (_ gobot_) _get_file() (*os.File, error) {
	id := os.Getenv("GODISC_INSTANCE_ID")
	f, err := os.OpenFile("./tmp/info-" + id + ".txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	return f, err
}

func (d gobot_) SetColor(color uint) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_color=" + strconv.FormatUint(uint64(color), 16) + "\n")
}

func (d gobot_) SetText(text string) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_description=" + text + "\n")
}

func (d gobot_) SetTitle(title string, url string) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_title=" + title + "\n")
	io.WriteString(f, "embed_url=" + url + "\n")
}

func (d gobot_) AddField(title string, text string, inline bool) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_field=" + title + "\u200b" + fmt.Sprint(inline) + "\u200b" + text + "\n")
}

func (d gobot_) SetTimestamp(timestamp time.Time) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	timestamp = timestamp.UTC()
	io.WriteString(f, "embed_timestamp=" + timestamp.Format("2006-01-02T15:04:05Z") + "\n")
}

func (d gobot_) SetFooter(text string, iconUrl string) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_footer=" + text + "\u200b" + iconUrl + "\n")
}

func (d gobot_) SetImage(url string, width int, height int) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_image=" + url + "\u200b" + fmt.Sprint(width) + "\u200b"+ fmt.Sprint(height) + "\n")
}

func (d gobot_) SetThumbnail(url string, width int, height int) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_thumbnail=" + url + "\u200b" + fmt.Sprint(width) + "\u200b"+ fmt.Sprint(height) + "\n")
}

func (d gobot_) SetVideo(url string, width int, height int) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_video=" + url + "\u200b" + fmt.Sprint(width) + "\u200b"+ fmt.Sprint(height) + "\n")
}

func (d gobot_) SetProvider(name string, url string) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_provider=" + name + "\u200b" + url + "\n")
}

func (d gobot_) SetAuthor(name string, url string, iconUrl string) {
	f, err := d._get_file()
	if err != nil {
		return
	}
	defer f.Close()

	io.WriteString(f, "embed_author=" + name + "\u200b" + url + "\u200b"+ iconUrl + "\n")
}

var gobot gobot_
`

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
	{"io", "Copy"},
	{"io/ioutil", "ReadAll"},
	{"os", "Exit"},
	{"net/http", "Get"},
	{"math", "Round"},
	{"math/big", "NewInt"},
	{"math/rand", "Int"},
	{"time", "Now"},
	{"strings", "Contains"},
	{"strconv", "Itoa"},
}

// 1: short, 1 backtick
// 3: long, 3 backticks
func codeFilter(code string, type_ int) string {
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

%s

func main() {
	%s
	rand.Seed(time.Now().UnixNano())
	fmt.Println(%s)
}`, imports, API, uses, code)
	}
	if strings.Index(code, "func main") == -1 {
		code = fmt.Sprintf(`package main

import (
	%s
)
%s


func main() {
	%s
	rand.Seed(time.Now().UnixNano())
	%s
}`, imports, API, uses, code)
	} else if strings.Index(code, "package") == -1 {
		code = fmt.Sprintf(`package main

import ( 
	%s
)

%s

func init() {
	%s
	rand.Seed(time.Now().UnixNano())
}

%s
`, imports, API, uses, code)
	} else {
		return ""
	}

	return code
}
