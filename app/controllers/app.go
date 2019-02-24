package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/revel/revel"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"

	"github.com/binkkatal/revel-websokets-chat/app/models"
)

const (
	REDIRECT_URL = "http://localhost:3000/Application/Auth"
)

type Application struct {
	*revel.Controller
}

var FACEBOOK = &oauth2.Config{
	ClientID:     os.Getenv("FB_CLIENT_ID"),
	ClientSecret: os.Getenv("FB_CLIENT_SECRET"),
	Scopes:       []string{},
	Endpoint:     facebook.Endpoint,
	RedirectURL:  REDIRECT_URL,
}

func (c Application) Index() revel.Result {
	u := c.connected()
	me := map[string]interface{}{}
	if u != nil && u.AccessToken != "" {
		resp, _ := http.Get("https://graph.facebook.com/me?access_token=" +
			url.QueryEscape(u.AccessToken))
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
			c.Log.Error("json decode error", "error", err)
		}
		c.Log.Info("Data fetched", "data", me)
	}
	authUrl := FACEBOOK.AuthCodeURL("state", oauth2.AccessTypeOffline)
	return c.Render(authUrl, me)
}

func (c Application) Destroy() {
	c.Controller.Destroy()
}

func (c Application) Auth(code string) revel.Result {
	tok, err := FACEBOOK.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.Log.Error("Exchange error", "error", err)
		return c.Redirect(Application.Index)
	}

	user := c.connected()
	user.AccessToken = tok.AccessToken
	return c.Redirect(Application.Index)
}

func (c Application) EnterDemo(user string) revel.Result {
	c.Validation.Required(user)

	if c.Validation.HasErrors() {
		c.Flash.Error("Please choose a nick name and the demonstration type.")
		return c.Redirect(Application.Index)
	}
	return c.Redirect("/websocket/room?user=%s", user)
}

func setUser(c *revel.Controller) revel.Result {
	var user *models.User
	if _, ok := c.Session["uid"]; ok {
		uid, _ := strconv.ParseInt(c.Session["uid"].(string), 10, 0)
		user = models.GetUser(int(uid))
	}
	if user == nil {
		user = models.NewUser()
		c.Session["uid"] = fmt.Sprintf("%d", user.Uid)
	}
	c.ViewArgs["user"] = user
	return nil
}

func (c Application) LogOut() revel.Result {
	c.Session.Del("uid")
	c.Session.Del("user")
	return c.Redirect("/")
}

func init() {
	revel.InterceptFunc(setUser, revel.BEFORE, &Application{})
}

func (c Application) connected() *models.User {
	return c.ViewArgs["user"].(*models.User)
}
