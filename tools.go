package goinside

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// regex
var (
	urlRe = regexp.MustCompile(`id=([^&\s]+)(?:&no=([^&\s]+))?(?:&page=([^&\s]+))?`)
)

// errors
var (
	errUnknownCause = errors.New("result false with empty cause")
)

// formatting
const (
	mobileGallURLFormat    = "http://m.dcinside.com/list.php?id=%v"
	mobileArticleURLFormat = "http://m.dcinside.com/view.php?id=%v&no=%v"
	gallogURLFormat        = "http://gallog.dcinside.com/%v"
	imageElementFormat     = `<img src="%v">`
	audioElementFormat     = `<audio controls><source src="%v" type="audio/mpeg">Your browser does not support the audio element.</audio>`
)

var (
	articleIconURLMap = map[ArticleType]string{
		TextArticleType:      "http://nstatic.dcinside.com/dgn/gallery/images/update/icon_text.png",
		TextBestArticleType:  "http://nstatic.dcinside.com/dgn/gallery/images/update/icon_text_b.png",
		ImageArticleType:     "http://nstatic.dcinside.com/dgn/gallery/images/update/icon_picture.png",
		ImageBestArticleType: "http://nstatic.dcinside.com/dgn/gallery/images/update/icon_picture_b.png",
		MovieArticleType:     "http://nstatic.dcinside.com/dgn/gallery/images/update/icon_movie.png",
		SuperBestArticleType: "http://nstatic.dcinside.com/dgn/gallery/images/update/sec_icon.png",
	}
	gallogIconURLMap = map[MemberType]string{
		HalfMemberType: "http://wstatic.dcinside.com/gallery/skin/gallog/g_fix.gif",
		FullMemberType: "http://wstatic.dcinside.com/gallery/skin/gallog/g_default.gif",
	}
)

func articleType(hasImage, isBest string) ArticleType {
	switch {
	case hasImage == "Y" && isBest == "Y":
		return ImageBestArticleType
	case hasImage == "Y" && isBest == "N":
		return ImageArticleType
	case hasImage == "N" && isBest == "Y":
		return TextBestArticleType
	case hasImage == "N" && isBest == "N":
		return TextArticleType
	}
	return UnknownArticleType
}

func commentType(dccon, voice string) CommentType {
	switch {
	case dccon == "" && voice == "":
		return TextCommentType
	case dccon != "" && voice == "":
		return DCconCommentType
	case dccon == "" && voice != "":
		return VoiceCommentType
	}
	return UnknownCommentType
}

func articleNumber(URL string) string {
	if urlRe.MatchString(URL) == false {
		return ""
	}
	return urlRe.FindStringSubmatch(URL)[2]
}

func articleURL(id, number string) string {
	return fmt.Sprintf(mobileArticleURLFormat, id, number)
}

func gallogURL(id string) string {
	if id == "" {
		return ""
	}
	return fmt.Sprintf(gallogURLFormat, id)
}

func gallURL(gallID string) string {
	return fmt.Sprintf(mobileGallURLFormat, gallID)
}

func gallID(URL string) string {
	if urlRe.MatchString(URL) == false {
		return ""
	}
	return urlRe.FindStringSubmatch(URL)[1]
}

func mustAtoi(a string) int {
	i, err := strconv.Atoi(a)
	if err != nil {
		return 0
	}
	return i
}

func toImageElement(c string) string {
	return fmt.Sprintf(imageElementFormat, c)
}

func toAudioElement(c string) string {
	return fmt.Sprintf(audioElementFormat, c)
}

// Maybe it's an unnecessary function now. This function was created when fetch data through web parsing.
func dateFormatter(s string) time.Time {
	if len(s) <= 5 {
		now := time.Now()
		s = fmt.Sprintf("%04d.%02d.%02d %v", now.Year(), int(now.Month()), now.Day(), s)
	}
	if t, err := time.Parse("2006.01.02", s); err == nil {
		return t
	}
	if t, err := time.Parse("2006.01.02 15:04", s); err == nil {
		return t
	}
	return time.Time{}
}

func removeNonPrintableUnicode(b []byte) []byte {
	buf := bytes.Buffer{}
	for _, v := range string(b) {
		if unicode.IsPrint(v) {
			buf.WriteRune(v)
		}
	}
	return buf.Bytes()
}

func checkResponse(resp *http.Response) error {
	jsonResp := make(jsonValidation, 1)
	if err := responseUnmarshal(resp, &jsonResp); err != nil {
		return err
	}
	return checkJSONResultTight(&jsonResp)
}

func responseUnmarshal(resp *http.Response, datas ...interface{}) error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	body = bytes.Replace(body, []byte(`\'`), []byte(`'`), -1)
	body = removeNonPrintableUnicode(body)
	for _, data := range datas {
		if err := json.Unmarshal(body, data); err != nil {
			r := strings.NewReplacer(
				"\\", `\\`,
				"\b", `\b`,
				"\f", `\f`,
				"\n", `\n`,
				"\r", `\r`,
				"\t", `\t`,
			)
			replaced := []byte(r.Replace(string(body)))
			if err := json.Unmarshal(replaced, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkJSONResultTight(jsonResp *jsonValidation) error {
	valid := (*jsonResp)[0]
	if valid.Result == false {
		if valid.Cause != "" {
			return errors.New(valid.Cause)
		}
		return errUnknownCause
	}
	return nil
}

func checkJSONResult(jsonResp *jsonValidation) error {
	valid := (*jsonResp)[0]
	if valid.Result == false && valid.Cause != "" {
		return errors.New(valid.Cause)
	}
	return nil
}

func makeForm(m map[string]string) io.Reader {
	data := url.Values{}
	for k, v := range m {
		data.Set(k, v)
	}
	return strings.NewReader(data.Encode())
}

func multipartForm(m map[string]string, images ...string) (io.Reader, string) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	if len(images) != 0 {
		multipartImages(w, images...)
		for i := range images {
			k := fmt.Sprintf("memo_block[%d]", i)
			m[k] = fmt.Sprintf("Dc_App_Img_%d", i+1)
		}
	}
	k := fmt.Sprintf("memo_block[%d]", len(images))
	m[k] = m["content"]
	delete(m, "content")
	multipartOthers(w, m)
	return &b, w.FormDataContentType()
}

func multipartImages(w *multipart.Writer, images ...string) {
	for i, image := range images {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="upload[%d]"; filename="%s"`, i, image))
		h.Set("Content-Type", "image/jpeg")
		fw, err := w.CreatePart(h)
		if err != nil {
			return
		}
		f, err := os.Open(image)
		if err != nil {
			return
		}
		if _, err = io.Copy(fw, f); err != nil {
			return
		}
	}
}

func multipartOthers(w *multipart.Writer, m map[string]string) {
	for k, v := range m {
		if fw, err := w.CreateFormField(k); err != nil {
			continue
		} else if _, err := fw.Write([]byte(v)); err != nil {
			continue
		}
	}
}
