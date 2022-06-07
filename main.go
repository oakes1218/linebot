package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/joho/godotenv/autoload"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

var (
	bot     *linebot.Client
	tgbot   *tgbotapi.BotAPI
	tbotErr error
	botErr  error
	loc     *time.Location
	client  = &http.Client{}
)

const (
	chatID  = 193618166
	tgToken = "1394548836:AAHdBSpf4QnA5Rt7xsLInEFDLMZ6i41Z0fY"
)

// 資料存放記憶體
var (
	sMg = make([]MemGroup, 0)
	sA  = make([]Activity, 0)
)

type MemGroup struct {
	Member string
	Date   string
	Clock  string
	Number string
}

type Activity struct {
	Number int64
	Name   string
	Date   string
	Times  string
}

func SetWeekGroup(mem, dt, ck, id string) (mg MemGroup) {
	mg.Member = mem
	mg.Date = dt
	mg.Clock = ck
	mg.Number = id

	return mg
}

// bot.ReplyMessage抽成func
func reply(event *linebot.Event, sentMsg ...linebot.SendingMessage) {
	if _, err := bot.ReplyMessage(event.ReplyToken, sentMsg...).Do(); err != nil {
		log.Println(err.Error())
		sendMsg("reply error : " + err.Error())
	}
}

func schedule(dateTime string, event *linebot.Event, sentMsg ...linebot.SendingMessage) {
	tt, err := time.ParseInLocation("2006-01-02 15:04:05", dateTime+":00", loc)
	if err != nil {
		log.Println(err.Error())
		sendMsg("schedule error : " + err.Error())
	}
	// 設定提醒清除排程
	go func() {
		stopTime := (tt.Unix() - time.Now().In(loc).Unix())
		log.Println("==================================")
		log.Println(tt.Unix())
		log.Println(stopTime)
		log.Println(stopTime + 60)
		log.Println("==================================")
		//過期十分鐘自動刪除
		time.Sleep((time.Duration(stopTime) + 60) * time.Second)
		log.Println("----------------------------------")
		log.Println(tt.Unix())
		log.Println("----------------------------------")
		for k, v := range sA {
			if tt.Unix() == v.Number {
				sA = append(sA[:k], sA[k+1:]...)
			}
		}

		for k, v := range sMg {
			if strconv.FormatInt(tt.Unix(), 10) == v.Number {
				sMg = append(sMg[:k], sMg[k+1:]...)
			}
		}
		sendMsg("刪除逾時活動")
		// LOOP:
		// 	for {
		// 		for k, v := range sA {
		// 			if tt.Unix()-60*60 == time.Now().Unix() {
		// 				reply(event, sentMsg...)
		// 			}

		// 			if tt.Unix() == v.Number {
		// 				sA = append(sA[:k], sA[k+1:]...)
		// 			}
		// 		}

		// 		for k, v := range sMg {
		// 			if strconv.FormatInt(tt.Unix(), 10) == v.Number {
		// 				sMg = append(sMg[:k], sMg[k+1:]...)
		// 				sendMsg("刪除逾時活動")
		// 				break LOOP
		// 			}
		// 		}
		// 	}
	}()
}

func sendMsg(msg string) {
	newMsg := tgbotapi.NewMessage(chatID, msg)
	_, err := tgbot.Send(newMsg)
	if err != nil {
		log.Println(err.Error())
		return
	}
}

func memList() string {
	var tital, msg, allmsg string
	for _, v := range sA {
		tital += "活動名稱 : " + v.Name + " 時間 : " + v.Date + " " + v.Times + " \n"
		for _, v1 := range sMg {
			if v.Date == v1.Date && v.Times == v1.Clock && strconv.FormatInt(v.Number, 10) == v1.Number {
				msg += "人員 : " + v1.Member + " \n"
			}
		}
		allmsg += tital
		allmsg += "======================\n"
		allmsg += msg
		allmsg += "\n"
		tital = ""
		msg = ""
	}

	return allmsg
}

func actList() *linebot.TemplateMessage {
	var cc []*linebot.CarouselColumn
	picture := "https://upload.cc/i1/2022/06/01/1ryUBP.jpeg"
	for _, v := range sA {
		cc = append(cc, linebot.NewCarouselColumn(
			picture,
			v.Date+" "+v.Times,
			v.Name,
			linebot.NewPostbackAction("參加", v.Date+"&"+v.Times+"&參加&"+v.Name+"&"+strconv.FormatInt(v.Number, 10), "", "", "", ""),
			linebot.NewPostbackAction("取消", v.Date+"&"+v.Times+"&取消&"+v.Name+"&"+strconv.FormatInt(v.Number, 10), "", "", "", ""),
			linebot.NewPostbackAction("刪除活動", strconv.FormatInt(v.Number, 10)+"&刪除&"+v.Date+"&"+v.Times+"&"+v.Name, "", "", "", ""),
		))
	}

	template := linebot.NewCarouselTemplate(cc...)
	msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)

	return msg
}

func main() {
	//server重啟發tg
	tgbot, tbotErr = tgbotapi.NewBotAPI(tgToken)
	if tbotErr != nil {
		log.Panic(tbotErr)
	}

	tgbot.Debug = true

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-quit:
			sendMsg("line bot重啟...")
		}
	}()

	//設定時區 timer定時喚醒heroku
	loc, _ = time.LoadLocation("Asia/Taipei")
	ticker := time.NewTicker(9 * 60 * time.Second)
	defer ticker.Stop()
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

// 防止heroku休眠
func runtime(ticker *time.Ticker, client *http.Client) {
	for range ticker.C {
		resp, err := client.Get("https://linesebot.herokuapp.com/ping")
		if err != nil {
			log.Println(err.Error())
		}

		if resp.StatusCode == 200 {
			log.Println("喚醒heroku")
		} else {
			sitemap, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
				return
			}
			log.Println(string(sitemap))
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
			//判斷群組或個人取使用者名稱
			var userName string
			if event.Source.GroupID == "" {
				res, err := bot.GetProfile(event.Source.UserID).Do()
				if err != nil {
					log.Println(err.Error())
				}
				userName = res.DisplayName
			} else {
				res, err := bot.GetGroupMemberProfile(event.Source.GroupID, event.Source.UserID).Do()
				if err != nil {
					log.Println(err.Error())
				}
				userName = res.DisplayName
			}

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

					reply(event, linebot.NewTextMessage(userName+" "+str[1]+" 活動 : "+str[4]+" 時段 : "+str[2]+" "+str[3]), actList())
					return
				}

				for k, v := range sMg {
					if v.Member == userName && v.Date == str[0] && v.Clock == str[1] && v.Number == str[4] {
						if str[2] == "參加" {
							return
						} else if str[2] == "取消" {
							sMg = append(sMg[:k], sMg[k+1:]...)
							reply(event, linebot.NewTextMessage(userName+" "+str[2]+" 活動 : "+str[3]+" 時段 : "+str[0]+" "+str[1]), linebot.NewTextMessage(memList()))
							return
						}
					}
				}

				if str[2] == "取消" {
					return
				}

				wg := SetWeekGroup(userName, str[0], str[1], str[4])
				sMg = append(sMg, wg)
				reply(event, linebot.NewTextMessage(userName+" "+str[2]+" 活動 : "+str[3]+" 時段 : "+str[0]+" "+str[1]), linebot.NewTextMessage(memList()))
			}
		case linebot.EventTypeMessage:
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				if message.Text == "功能列表" {
					leftBtn := linebot.NewMessageAction("查看活動", "查看活動")
					rightBtn := linebot.NewMessageAction("參加人員", "參加人員")
					template := linebot.NewConfirmTemplate("新增活動指令： \n格式 ： date&time&activity \nex. 2022-01-01&00:00&散步步", leftBtn, rightBtn)
					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)
					reply(event, msg)
				}

				if message.Text == "LoG" {
					s, err := json.Marshal(sMg)
					if err != nil {
						log.Printf("Error: %s", err)
						sendMsg("json.Marshal err : " + err.Error())
						return
					}

					sa, err := json.Marshal(sA)
					if err != nil {
						log.Printf("Error: %s", err)
						sendMsg("json.Marshal err : " + err.Error())
						return
					}

					reply(event, linebot.NewTextMessage(string(s)), linebot.NewTextMessage(string(sa)))
				}

				if message.Text == "clearAll" {
					sMg = sMg[:0]
					sA = sA[:0]
					reply(event, linebot.NewTextMessage("Success clearAll"))
				}

				if message.Text == "參加人員" {
					if len(sMg) == 0 {
						reply(event, linebot.NewTextMessage("無參加人員"))
					} else {
						reply(event, linebot.NewTextMessage(memList()))
					}
				}

				if message.Text == "查看活動" {
					if len(sA) == 0 {
						reply(event, linebot.NewTextMessage("無活動列表"))
						return
					}
					reply(event, actList())
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
							tt, err := time.ParseInLocation("2006-01-02 15:04:05", sa[0]+" "+sa[1]+":00", loc)
							if err != nil {
								sendMsg("ParseInLocation err : " + err.Error())
							}

							ac.Number = tt.Unix()
							ac.Name = sa[2]
							ac.Date = sa[0]
							ac.Times = sa[1]
							sA = append(sA, ac)
							msg += "新增活動成功"
							schedule(sa[0]+" "+sa[1], event, linebot.NewTextMessage("溫馨提醒 : "+sa[2]+"活動一小時後開始"))
						}
						reply(event, linebot.NewTextMessage(msg), actList())
					}
				}
			}
		default:
			log.Printf("Unknown event: %v", event)
			sendMsg("Unknown event")
		}
	}
}
