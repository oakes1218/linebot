package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
	date   string = "2022-06-17"
	times  string = "18:00"
	date2  string = "2022-06-17"
	times2 string = "19:00"
)

var (
	bot *linebot.Client
	err error
	sWg = make([]WeekGroup, 0)
	loc *time.Location
)

type WeekGroup struct {
	Week      string    `json: "week"`
	Clock     string    `json: "clock"`
	Member    string    `json: "member"`
	TimesTamp time.Time `json "times_tamp"`
}

func SetWeekGroup(mem, wk, ck string) (wg WeekGroup) {
	wg.Member = mem
	wg.Week = wk
	wg.Clock = ck
	wg.TimesTamp = getTime()

	return wg
}

func main() {
	loc, _ = time.LoadLocation("Asia/Taipei")
	ticker := time.NewTicker(28 * 60 * time.Second)
	defer ticker.Stop()
	client := &http.Client{}
	go runtime(ticker, client)

	bot, err = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))

	if err != nil {
		log.Println(err.Error())
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

func getTime() time.Time {
	return time.Now().In(loc)
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
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				if message.Text == "cmd" {
					leftBtn := linebot.NewMessageAction("查看活動", "查看活動")
					rightBtn := linebot.NewMessageAction("參加人員", "參加人員")
					template := linebot.NewConfirmTemplate("選擇指令", leftBtn, rightBtn)
					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)

					if _, err = bot.ReplyMessage(event.ReplyToken, msg).Do(); err != nil {
						log.Println(err.Error())
					}
				}

				if message.Text == "LoG" {
					s, err := json.Marshal(sWg)
					if err != nil {
						fmt.Printf("Error: %s", err)
						return
					}
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(string(s))).Do(); err != nil {
						log.Println(err.Error())
					}
				}

				if message.Text == "參加人員" {
					if len(sWg) == 0 {
						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("無參加人員")).Do(); err != nil {
							log.Println(err.Error())
						}
					} else {
						var tital1, tital2, msg1, msg2, allmsg string
						for _, v := range sWg {
							if v.Clock == times && v.Week == date {
								tital1 = " 時間: " + v.Week + " " + v.Clock + " \n"
								msg1 += "人員: " + v.Member + " \n"
							}
							if v.Clock == times2 && v.Week == date2 {
								tital2 = " 時間: " + v.Week + " " + v.Clock + " \n"
								msg2 += "人員: " + v.Member + " \n"
							}
						}

						allmsg = tital1 + msg1 + tital2 + msg2
						if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(allmsg)).Do(); err != nil {
							log.Println(err.Error())
						}
					}
				}

				if message.Text == "查看活動" {
					res, err := bot.GetProfile(event.Source.UserID).Do()
					if err != nil {
						log.Println(err.Error())
					}

					template := linebot.NewCarouselTemplate(
						linebot.NewCarouselColumn(
							"https://upload.cc/i1/2022/06/01/1ryUBP.jpeg",
							date+" "+times,
							"好韻健身房",
							linebot.NewPostbackAction("參加", date+"&"+times+"&參加&"+res.DisplayName, "", res.DisplayName+"參加"+date+" "+times+" 時段", "", ""),
							linebot.NewPostbackAction("取消", date+"&"+times+"&取消&"+res.DisplayName, "", res.DisplayName+"取消"+date+" "+times+" 時段", "", ""),
						),
						linebot.NewCarouselColumn(
							"https://upload.cc/i1/2022/06/01/1ryUBP.jpeg",
							date2+" "+times2,
							"好韻健身房",
							linebot.NewPostbackAction("參加", date2+"&"+times2+"&參加&"+res.DisplayName, "", res.DisplayName+"參加"+date2+" "+times2+" 時段", "", ""),
							linebot.NewPostbackAction("取消", date2+"&"+times2+"&取消&"+res.DisplayName, "", res.DisplayName+"取消"+date2+" "+times2+" 時段", "", ""),
						))

					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)
					if _, err = bot.ReplyMessage(event.ReplyToken, msg).Do(); err != nil {
						log.Println(err.Error())
					}
				}
			}
		}

		if event.Postback.Data != "" {
			str := strings.Split(event.Postback.Data, "&")
			for k, v := range sWg {
				if v.Member == str[3] && v.Week == str[0] && v.Clock == str[1] {
					if str[2] == "參加" {
						return
					} else if str[2] == "取消" {
						sWg = append(sWg[:k], sWg[k+1:]...)
						return
					}
				}
			}
			if str[2] == "取消" {
				return
			}

			wg := SetWeekGroup(str[3], str[0], str[1])
			sWg = append(sWg, wg)
		}
	}
}
