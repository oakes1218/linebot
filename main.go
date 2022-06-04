package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

const (
	MON = "Monday"
	TUE = "Tuseday"
	WED = "wednesday"
	THU = "Thursday"
	FRI = "Firday"
	SAT = "Saturday"
	SUN = "Sunday"
)

var (
	bot    *linebot.Client
	botErr error
	sMg    = make([]MemGroup, 0)
	sA     = make([]Activity, 0)
	loc    *time.Location
)

type MemGroup struct {
	Member string `json: "member"`
	Date   string `json: "date"`
	Clock  string `json: "clock"`
	Number string `json "number"`
}

type Activity struct {
	Number int64  `json: "nember"`
	Name   string `json: "name"`
	Date   string `json: "date"`
	Times  string `json: "date"`
}

func SetWeekGroup(mem, dt, ck, id string) (mg MemGroup) {
	mg.Member = mem
	mg.Date = dt
	mg.Clock = ck
	mg.Number = id

	return mg
}

func main() {
	loc, _ = time.LoadLocation("Asia/Taipei")
	ticker := time.NewTicker(28 * 60 * time.Second)
	defer ticker.Stop()
	client := &http.Client{}
	go runtime(ticker, client)

	bot, botErr = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))

	if botErr != nil {
		log.Println(botErr.Error())
		return
	}

	r := gin.Default()
	r.POST("/callback", callbackHandler)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run()
}

func runtime(ticker *time.Ticker, client *http.Client) {
	for {
		select {
		case <-ticker.C:
			resp, err := client.Get("https://linesebot.herokuapp.com/ping")
			defer resp.Body.Close()
			if err != nil {
				fmt.Println(err.Error())
			}

			if resp.StatusCode == 200 {
				fmt.Println("喚醒heroku")
			} else {
				fmt.Println(resp.Status)
				sitemap, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Fatal(err)
					return
				}
				fmt.Println(string(sitemap))
			}
		}
	}
}

func callbackHandler(c *gin.Context) {
	defer c.Request.Body.Close()
	// 接收請求
	events, err := bot.ParseRequest(c.Request)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		return
	}

	for _, event := range events {
		switch event.Type {
		case linebot.EventTypePostback:
			if event.Postback.Data != "" {
				str := strings.Split(event.Postback.Data, "&")
				if str[1] == "刪除" {
					for k, v := range sA {
						if strconv.FormatInt(v.Number, 10) == str[0] {
							sA = append(sA[:k], sA[k+1:]...)
						}
					}

					for k, v := range sMg {
						if v.Number == str[0] {
							sMg = append(sMg[:k], sMg[k+1:]...)
						}
					}
					return
				}

				for k, v := range sMg {
					if v.Member == str[3] && v.Date == str[0] && v.Clock == str[1] && v.Number == str[4] {
						if str[2] == "參加" {
							return
						} else if str[2] == "取消" {
							sMg = append(sMg[:k], sMg[k+1:]...)
							return
						}
					}
				}

				if str[2] == "取消" {
					return
				}

				wg := SetWeekGroup(str[3], str[0], str[1], str[4])
				sMg = append(sMg, wg)
			}
		case linebot.EventTypeMessage:
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				if message.Text == "cmd" {
					leftBtn := linebot.NewMessageAction("查看活動", "查看活動")
					rightBtn := linebot.NewMessageAction("參加人員", "參加人員")
					template := linebot.NewConfirmTemplate("新增活動指令： \n格式 ： date&time&activity \nex. 2022-01-01&00:00&散步步", leftBtn, rightBtn)
					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)

					if _, err = bot.ReplyMessage(event.ReplyToken, msg).Do(); err != nil {
						log.Println(err.Error())
					}
				}

				if message.Text == "LoG" {
					s, err := json.Marshal(sMg)
					if err != nil {
						fmt.Printf("Error: %s", err)
						return
					}
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(string(s))).Do(); err != nil {
						log.Println(err.Error())
					}
				}

				if message.Text == "clearAll" {
					sMg = sMg[:0]
					sA = sA[:0]
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Success clearAll")).Do(); err != nil {
						log.Println(err.Error())
					}
				}

				if message.Text != "" {
					var msg string
					sa := strings.Split(message.Text, "&")
					if len(sa) == 3 {
						dRegex := `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`
						tRegex := `^[0-9]{2}:[0-9]{2}$`
						rd, rErr := regexp.Compile(dRegex)
						rt, tErr := regexp.Compile(tRegex)
						if rErr != nil || tErr != nil {
							log.Println(err.Error())
							return
						}

						if !rd.MatchString(sa[0]) {
							msg += "日期格式錯誤 "
						}

						if !rt.MatchString(sa[1]) {
							msg += "時間格式錯誤 "
						}

						if msg == "" {
							var ac Activity
							ac.Number = time.Now().In(loc).Unix()
							ac.Name = sa[2]
							ac.Date = sa[0]
							ac.Times = sa[1]
							sA = append(sA, ac)
						}

						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(msg)).Do(); err != nil {
							log.Println(err.Error())
						}
					}
				}

				if message.Text == "參加人員" {
					if len(sMg) == 0 {
						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("無參加人員")).Do(); err != nil {
							log.Println(err.Error())
						}
					} else {
						var tital, msg, allmsg string
						for _, v := range sA {
							tital += "活動名稱 ： " + v.Name + " 時間: " + v.Date + " " + v.Times + " \n"
							for _, v1 := range sMg {
								if v.Date == v1.Date && v.Times == v1.Clock && strconv.FormatInt(v.Number, 10) == v1.Number {
									msg += "人員: " + v1.Member + " \n"
								}
							}
							allmsg += tital
							allmsg += "========================"
							allmsg += msg
							tital = ""
							msg = ""
						}

						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(allmsg)).Do(); err != nil {
							log.Println(err.Error())
						}
					}
				}

				if message.Text == "查看活動" {
					if len(sA) == 0 {
						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("無活動列表")).Do(); err != nil {
							log.Println(err.Error())
						}
						return
					}

					var cc []*linebot.CarouselColumn
					picture := "https://upload.cc/i1/2022/06/01/1ryUBP.jpeg"
					res, err := bot.GetProfile(event.Source.UserID).Do()
					if err != nil {
						log.Println(err.Error())
					}

					for _, v := range sA {
						cc = append(cc, linebot.NewCarouselColumn(
							picture,
							v.Date+" "+v.Times,
							v.Name,
							linebot.NewPostbackAction("參加", v.Date+"&"+v.Times+"&參加&"+res.DisplayName+"&"+strconv.FormatInt(v.Number, 10), "", res.DisplayName+" 參加： "+v.Name+" \n時段："+v.Date+" "+v.Times, "", ""),
							linebot.NewPostbackAction("取消", v.Date+"&"+v.Times+"&取消&"+res.DisplayName+"&"+strconv.FormatInt(v.Number, 10), "", res.DisplayName+" 取消： "+v.Name+" \n時段："+v.Date+" "+v.Times, "", ""),
							linebot.NewPostbackAction("刪除活動", strconv.FormatInt(v.Number, 10)+"&刪除", "", res.DisplayName+"刪除 活動 ： "+v.Name+" 時段 ： "+v.Date+" "+v.Times, "", ""),
						))
					}

					template := linebot.NewCarouselTemplate(cc...)

					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)
					if _, err = bot.ReplyMessage(event.ReplyToken, msg).Do(); err != nil {
						log.Println(err.Error())
					}
				}
			}
		default:
			log.Printf("Unknown event: %v", event)
		}
	}
}
