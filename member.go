package goinside

import (
	"fmt"
	"io"
	"time"
)

// MemberSession 구조체는 고정닉의 세션을 나타냅니다.
type MemberSession struct {
	id   string
	pw   string
	conn *Connection
	*MemberSessionDetail
	app *App
}

// MemberSessionDetail 구조체는 해당 세션의 세부 정보를 나타냅니다.
type MemberSessionDetail struct {
	UserID string `json:"user_id"`
	UserNO string `json:"user_no"`
	Name   string `json:"name"`
	Stype  string `json:"stype"`

	Result bool   `json:"result"`
	Cause  string `json:"cause"`
	IsBot  bool   `json:"is_bot"`
}

// Login 함수는 고정닉 세션을 반환합니다.
func Login(id, pw string) (ms *MemberSession, err error) {
	form := makeForm(map[string]string{
		"user_id": id,
		"user_pw": pw,
	})
	tempMS := &MemberSession{
		id:   id,
		pw:   pw,
		conn: &Connection{timeout: time.Second * 5},
	}
	resp, err := loginAPI.post(tempMS, form, defaultContentType)
	if err != nil {
		return
	}
	tempMSD := new([]MemberSessionDetail)
	err = responseUnmarshal(resp, tempMSD)
	if err != nil {
		return
	}
	if !(*tempMSD)[0].isSucceed() {
		err = fmt.Errorf("login fail: %v", (*tempMSD)[0].Cause)
		return
	}
	tempMS.MemberSessionDetail = &((*tempMSD)[0])

	valueToken, appID, err := FetchAppID(tempMS)
	if err != nil {
		return nil, err
	}
	tempMS.app = &App{Token: valueToken, ID: appID}

	ms = tempMS
	return
}

func (msd *MemberSessionDetail) isSucceed() bool {
	switch {
	case msd.UserID == "":
		return false
	case msd.UserNO == "":
		return false
	}
	return true
}

// Logout 메소드는 해당 고정닉 세션을 종료합니다.
func (ms *MemberSession) Logout() (err error) {
	ms = nil
	return
}

// Connection 메소드는 해당 세션의 Connection 구조체를 반환합니다.
func (ms *MemberSession) Connection() *Connection {
	if ms.conn == nil {
		ms.conn = &Connection{}
	}
	return ms.conn
}

// Write 메소드는 글이나 댓글과 같은 쓰기 가능한 객체를 전달받아 작성 요청을 보냅니다.
func (ms *MemberSession) Write(w writable) error {
	return w.write(ms)
}

func (ms *MemberSession) articleWriteForm(id, s, c string, is ...string) (io.Reader, string) {
	return multipartForm(map[string]string{
		"app_id":  ms.getAppID(),
		"mode":    "write",
		"user_id": ms.UserID,
		"id":      id,
		"subject": s,
		"content": c,
	}, is...)
}

func (ms *MemberSession) commentWriteForm(id, n, c string, is ...string) (io.Reader, string) {
	return multipartForm(map[string]string{
		"app_id":       ms.getAppID(),
		"user_id":      ms.UserID,
		"id":           id,
		"no":           n,
		"comment_memo": c,
		"mode":         "com_write",
	}, is...)
}

// Delete 메소드는 삭제 가능한 객체를 전달받아 삭제 요청을 보냅니다.
func (ms *MemberSession) Delete(d deletable) error {
	return d.delete(ms)
}

func (ms *MemberSession) articleDeleteForm(id, n string) (io.Reader, string) {
	return makeForm(map[string]string{
		"app_id":  ms.getAppID(),
		"user_id": ms.UserID,
		"no":      n,
		"id":      id,
		"mode":    "board_del",
	}), defaultContentType
}

func (ms *MemberSession) commentDeleteForm(id, n, cn string) (io.Reader, string) {
	return makeForm(map[string]string{
		"app_id":     ms.getAppID(),
		"user_id":    ms.UserID,
		"id":         id,
		"no":         n,
		"mode":       "comment_del",
		"comment_no": cn,
	}), defaultContentType
}

// ThumbsUp 메소드는 해당 글에 추천 요청을 보냅니다.
func (ms *MemberSession) ThumbsUp(a actionable) error {
	return a.thumbsUp(ms)
}

// ThumbsDown 메소드는 해당 글에 비추천 요청을 보냅니다.
func (ms *MemberSession) ThumbsDown(a actionable) error {
	return a.thumbsDown(ms)
}

func (ms *MemberSession) actionForm(id, n string) (io.Reader, string) {
	return makeForm(map[string]string{
		"app_id": ms.getAppID(),
		"id":     id,
		"no":     n,
	}), nonCharsetContentType
}

func (ms *MemberSession) getAppID() string {
	valueToken, err := GenerateValueToken()
	if err != nil {
		return ""
	}
	if ms.app.Token == valueToken {
		return ms.app.ID
	}
	valueToken, appID, err := FetchAppID(ms)
	if err != nil {
		return ""
	}
	ms.app = &App{Token: valueToken, ID: appID}
	return ms.app.ID
}
