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
	member string `json: "member"`
	date   string `json: "date"`
	clock  string `json: "clock"`
	number string `json "number"`
}

type Activity struct {
	number int64  `json: "nember"`
	name   string `json: "name"`
	date   string `json: "date"`
	times  string `json: "date"`
}

func SetWeekGroup(mem, dt, ck, id string) (mg MemGroup) {
	mg.member = mem
	mg.date = dt
	mg.clock = ck
	mg.number = id

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
				log.Println(111111)
				log.Println(sMg)
				log.Println(event.Postback.Data)
				str := strings.Split(event.Postback.Data, "&")
				if str[1] == "刪除" {
					for k, v := range sA {
						if strconv.FormatInt(v.number, 10) == str[0] {
							sA = append(sA[:k], sA[k+1:]...)
							return
						}
					}
				}

				for k, v := range sMg {
					if v.member == str[3] && v.date == str[0] && v.clock == str[1] && v.number == str[4] {
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
				log.Println(sMg)
			}
		case linebot.EventTypeMessage:
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				if message.Text == "cmd" {
					leftBtn := linebot.NewMessageAction("查看活動", "查看活動")
					rightBtn := linebot.NewMessageAction("參加人員", "參加人員")
					template := linebot.NewConfirmTemplate("新增活動指令： \n date&time&activity \n ex. 2022-01-01&00:00&散步步", leftBtn, rightBtn)
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
							msg += "日期格式錯誤 \n"
						}

						if !rt.MatchString(sa[1]) {
							msg += "時間格式錯誤 \n"
						}

						if msg == "" {
							var ac Activity
							res, err := bot.GetProfile(event.Source.UserID).Do()
							if err != nil {
								log.Println(err.Error())
							}

							ac.number = time.Now().In(loc).Unix()
							ac.name = sa[0]
							ac.date = sa[1]
							ac.times = sa[2]
							sA = append(sA, ac)
							msg = "ID : " + strconv.FormatInt(ac.number, 10) + " " + res.DisplayName + "新增活動 ： " + sa[3] + " 時間 ： " + sa[0] + " " + sa[1]
						}

						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(msg)).Do(); err != nil {
							log.Println(err.Error())
						}
					}
				}

				if message.Text == "參加人員" {
					log.Println(sA, sMg)
					if len(sMg) == 0 {
						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("無參加人員")).Do(); err != nil {
							log.Println(err.Error())
						}
					} else {
						var tital, msg, allmsg string
						for _, v := range sA {
							for _, v1 := range sMg {
								if v.date == v1.date && v.times == v1.clock && strconv.FormatInt(v.number, 10) == v1.number {
									tital = "活動 ： " + v.name + " 時間: " + v.date + " " + v.times + " \n"
									msg += "人員: " + v1.member + " \n"
								}
							}
						}

						allmsg = tital + msg
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

					// var cc []*linebot.CarouselColumn
					picture := "https://upload.cc/i1/2022/06/01/1ryUBP.jpeg"
					res, err := bot.GetProfile(event.Source.UserID).Do()
					if err != nil {
						log.Println(err.Error())
					}

					// for _, v := range sA {
					// cc = append(cc, linebot.NewCarouselColumn(
					// 	picture,
					// 	v.date+" "+v.times,
					// 	v.name,
					// 	linebot.NewPostbackAction("參加", v.date+"&"+v.times+"&參加&"+res.DisplayName+"&"+strconv.FormatInt(v.number, 10), "", res.DisplayName+"參加"+v.date+" "+v.times+" 時段", "", ""),
					// 	linebot.NewPostbackAction("取消", v.date+"&"+v.times+"&取消&"+res.DisplayName+"&"+strconv.FormatInt(v.number, 10), "", res.DisplayName+"取消"+v.date+" "+v.times+" 時段", "", ""),
					// 	linebot.NewPostbackAction("刪除活動", strconv.FormatInt(v.number, 10)+"&刪除", "", res.DisplayName+"刪除 活動 ： "+v.name+" 時段 ： "+v.date+" "+v.times, "", ""),
					// ))
					// }

					template := linebot.NewCarouselTemplate(linebot.NewCarouselColumn(
						picture,
						sA[0].date+" "+sA[0].times,
						sA[0].name,
						linebot.NewPostbackAction("參加", sA[0].date+"&"+sA[0].times+"&參加&"+res.DisplayName+"&"+strconv.FormatInt(sA[0].number, 10), "", res.DisplayName+"參加"+sA[0].date+" "+sA[0].times+" 時段", "", ""),
						linebot.NewPostbackAction("取消", sA[0].date+"&"+sA[0].times+"&取消&"+res.DisplayName+"&"+strconv.FormatInt(sA[0].number, 10), "", res.DisplayName+"取消"+sA[0].date+" "+sA[0].times+" 時段", "", ""),
						linebot.NewPostbackAction("刪除活動", strconv.FormatInt(sA[0].number, 10)+"&刪除", "", res.DisplayName+"刪除 活動 ： "+sA[0].name+" 時段 ： "+sA[0].date+" "+sA[0].times, "", ""),
					))

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
