package main

import (
	"encoding/json"
	"fmt"
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
	bot *linebot.Client
	err error
	sWg []WeekGroup
	loc *time.Location
)

type WeekGroup struct {
	Week   string `json: "week"`
	Clock  string `json: "clock"`
	Member string `json: "member"`
}

func SetWeekGroup(mem, wk, ck string) (wg WeekGroup) {
	wg.Member = mem
	wg.Week = wk
	wg.Clock = ck

	return wg
}

func main() {
	loc, _ = time.LoadLocation("Asia/Taipei")
	bot, err = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))

	if err != nil {
		log.Println(err.Error())
		return
	}

	r := gin.Default()
	r.POST("/callback", callbackHandler)
	r.Run()
}

func getWeek() string {
	t := time.Now().In(loc)
	return t.Weekday().String()
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
		timeStr := time.Now().In(loc).Format("15:04:05")
		st := strings.Split(timeStr, ":")
		//晚上十一點砍資料
		if st[0] == "23" {
			for k, v := range sWg {
				if v.Week == getWeek() {
					sWg = append(sWg[:k], sWg[k+1:]...)
				}
			}
		}

		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				// 回覆訊息
				if message.Text == "查看活動" {
					leftBtn := linebot.NewMessageAction("六點", SAT+" 六點")
					rightBtn := linebot.NewMessageAction("七點", SAT+" 七點")
					template := linebot.NewConfirmTemplate("選擇時間", leftBtn, rightBtn)
					// template := linebot.NewButtonsTemplate("https://www.facebook.com/win2fitness/photos/a.593850231091748/595671197576318/", "日期", "星期幾", leftBtn, rightBtn)
					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)

					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Get: "+message.Text+" , \n OK!"), msg).Do(); err != nil {
						log.Println(err.Error())
					}

				}

				if message.Text == SAT+" 六點" {
					res, err := bot.GetProfile(event.Source.UserID).Do()
					if err != nil {
						log.Println(err.Error())
					}

					str := strings.Split(message.Text, " ")
					wg := SetWeekGroup(res.DisplayName, str[0], str[1])
					sWg = append(sWg, wg)

					s, err := json.Marshal(sWg)
					if err != nil {
						fmt.Printf("Error: %s", err)
						return
					}

					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(string(s))).Do(); err != nil {
						log.Println(err.Error())
					}
				}

				if message.Text == MON+" 六點" {
					res, err := bot.GetProfile(event.Source.UserID).Do()
					if err != nil {
						log.Println(err.Error())
					}

					str := strings.Split(message.Text, " ")
					wg := SetWeekGroup(res.DisplayName, str[0], str[1])
					sWg = append(sWg, wg)

					s, err := json.Marshal(sWg)
					if err != nil {
						fmt.Printf("Error: %s", err)
						return
					}

					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(string(s))).Do(); err != nil {
						log.Println(err.Error())
					}
				}

				if message.Text == SAT+" 七點" {
					res, err := bot.GetProfile(event.Source.UserID).Do()
					if err != nil {
						log.Println(err.Error())
					}

					str := strings.Split(message.Text, " ")
					wg := SetWeekGroup(res.DisplayName, str[0], str[1])
					sWg = append(sWg, wg)

					s, err := json.Marshal(sWg)
					if err != nil {
						fmt.Printf("Error: %s", err)
						return
					}

					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(string(s))).Do(); err != nil {
						log.Println(err.Error())
					}
				}
			}
		}
	}
}
