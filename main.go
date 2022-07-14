package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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

	_ "github.com/lib/pq"
	"github.com/tidwall/gjson"
)

var (
	bot     *linebot.Client
	tgbot   *tgbotapi.BotAPI
	db      *sql.DB
	tbotErr error
	botErr  error
	dbErr   error
	loc     *time.Location
	client  = &http.Client{}
)

const (
	chatID  = 193618166
	tgToken = "1394548836:AAHdBSpf4QnA5Rt7xsLInEFDLMZ6i41Z0fY"
)

var (
	date        = "2022-08-11"
	times       = "19:30"
	p     int64 = 4
	// 台中新馬辣
	company  = "-LjL6vW09dVGOC0tGamg"
	branchID = "-Mw-S9FSZby-GN_0tiZe"
	// 台中中山燒肉
	// company  = -LzoSPyWXzTNSaE - I4QJ
	// branchID = -MdytTkuohNf5wnBz1vZ
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

type LineBot struct {
	MemList string
	ActList string
	kind    string
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

func schedule(dateTime string) {
	tt, err := time.ParseInLocation("2006-01-02 15:04:05", dateTime+":00", loc)
	if err != nil {
		log.Println(err.Error())
		sendMsg("schedule error : " + err.Error())
	}
	// 設定提醒清除排程
	go func() {
		stopTime := (tt.Unix() - time.Now().In(loc).Unix())
		//過期十分鐘自動刪除
		time.Sleep((time.Duration(stopTime) + 60) * time.Second)
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
	picture := "https://upload.cc/i1/2022/06/10/9yF8Lh.jpg"
	if len(sA) == 0 {
		return nil
	}

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

func getDB() (m []MemGroup, a []Activity) {
	var lb LineBot
	rows, err := db.Query("SELECT * FROM linebot")
	if err != nil {
		sendMsg("db.Query error : " + err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&lb.MemList, &lb.ActList, &lb.kind); err != nil {
			sendMsg("rows.Next error : " + err.Error())
			return
		}
	}

	mErr := json.Unmarshal([]byte(lb.MemList), &m)
	aErr := json.Unmarshal([]byte(lb.ActList), &a)
	if mErr != nil || aErr != nil {
		sendMsg("Unmarshal error : " + mErr.Error() + aErr.Error())
		return
	}

	sendMsg("人員列表 : " + lb.MemList)
	sendMsg("活動列表 : " + lb.ActList)

	return m, a
}

func updateMemDB(m string) {
	kind := "line"
	// m := `[{"Member":"Eddie","Date":"2022-06-15","Clock":"17:30","Number":"1654361276"},{"Member":"刁","Date":"2022-06-15","Clock":"17:30","Number":"1654361276"},{"Member":"陸澤宇","Date":"2022-06-15","Clock":"17:30","Number":"1654361276"},{"Member":"Steve","Date":"2022-06-15","Clock":"17:30","Number":"1654361276"},{"Member":"Eddie","Date":"2022-06-15","Clock":"17:30","Number":"1654614480"},{"Member":"刁","Date":"2022-06-15","Clock":"17:30","Number":"1654614480"},{"Member":"陸澤宇","Date":"2022-06-15","Clock":"17:30","Number":"1654614480"},{"Member":"Steve","Date":"2022-06-15","Clock":"17:30","Number":"1654614480"},{"Member":"Momo","Date":"2022-06-15","Clock":"17:30","Number":"1654614480"}]`
	// a := `[{"Number":1654361276,"Name":"chill play","Date":"2022-06-15","Times":"17:30"},{"Number":1654614480,"Name":"jim 生日","Date":"2022-06-10","Times":"21:30:"}]`
	_, err := db.Exec("UPDATE linebot SET MemList=$1 WHERE kind=$2", m, kind)
	if err != nil {
		sendMsg("updateMemDB error : " + err.Error())
		return
	}
}

func updateActDB(a string) {
	kind := "line"
	_, err := db.Exec("UPDATE linebot SET ActList=$1 WHERE kind=$2", a, kind)
	if err != nil {
		sendMsg("updateActDB error : " + err.Error())
		return
	}
}

func logMemList() string {
	s, err := json.Marshal(sMg)
	if err != nil {
		log.Printf("Error: %s", err)
		sendMsg("json.Marshal err : " + err.Error())
		return ""
	}

	return string(s)
}

func logActList() string {
	s, err := json.Marshal(sA)
	if err != nil {
		log.Printf("Error: %s", err)
		sendMsg("json.Marshal err : " + err.Error())
		return ""
	}

	return string(s)
}

func inline() {
	t := time.NewTicker(time.Second * 10)
	defer t.Stop()
	uri, err := url.Parse("https://www.89ip.cn/")
	if err != nil {
		log.Println(err)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(uri),
		},
	}

loop:
	for {
		<-t.C
		url := "https://inline.app/api/booking-capacitiesV3?companyId=" + company + "%3Ainline-live-1&branchId=" + branchID
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36")
		if err != nil {
			sendMsg("http.NewRequest error :" + err.Error())
			break loop
		}

		resp, err := client.Do(req)
		if err != nil {
			sendMsg("client.Do error :" + err.Error())
			break loop
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			sendMsg("ioutil.ReadAll error :" + err.Error())
			break loop
		}

		if resp.Status != "200 OK" {
			sendMsg("抓取網頁錯誤")
			sendMsg(resp.Status)
			sendMsg(string(body))
			break loop
		}

		val := gjson.Get(string(body), "default."+date+".times."+times).Array()
		if len(val) != 0 {
			sendMsg("空缺位置" + gjson.Get(string(body), "default."+date+".times."+times).String())
		}

		for _, v := range val {
			if v.Int() == p {
				url := "https://inline.app/api/reservations/booking"
				jsonData := []byte(`{"language":"zh-tw","company":"` + company + `:inline-live-1","branch":"` + branchID + `","groupSize":"` + strconv.FormatInt(p, 10) + `","kids":0,"gender":0,"purposes":[],"email":"","name":"李亞諦","phone":"+886937550247","note":"","date":"` + date + `","time":"` + times + `","numberOfKidChairs":0,"numberOfKidSets":0,"skipPhoneValidation":false,"referer":"www.google.com"}`)
				req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
				req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36")
				req.Header.Set("Content-Type", "application/json")
				if err != nil {
					sendMsg("http.NewRequest error :" + err.Error())
					break loop
				}

				resp, err := client.Do(req)
				if err != nil {
					sendMsg("client.Do error :" + err.Error())
					break loop
				}
				defer resp.Body.Close()

				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					sendMsg("ioutil.ReadAll error :" + err.Error())
					break loop
				}

				sendMsg("訂位成功" + string(body))
				break loop
			}
		}
	}
	sendMsg("訂位程序中斷")
}

func main() {
	//server重啟發tg
	tgbot, tbotErr = tgbotapi.NewBotAPI(tgToken)
	if tbotErr != nil {
		log.Panic(tbotErr)
	}

	tgbot.Debug = true

	go func() {
		quit := make(chan os.Signal, 1)
		errs := make(chan error, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-quit)
		sendMsg("line bot重啟...")

		c := <-errs
		sendMsg("中斷訊號 : " + c.Error())
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}()

	//設定時區 timer定時喚醒heroku
	loc, _ = time.LoadLocation("Asia/Taipei")
	ticker := time.NewTicker(9 * 60 * time.Second)
	defer ticker.Stop()
	go runtime(ticker, client)
	go inline()

	bot, botErr = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))
	if botErr != nil {
		log.Println(botErr.Error())
		return
	}

	connStr := "postgres://dpwuoblyktjbyx:e793be9f374787e9039852fadcc0d0c4cf2a9f4f44479fd865e65f45f309f93c@ec2-3-219-229-143.compute-1.amazonaws.com:5432/dc3ghn65mhd6gn"
	db, dbErr = sql.Open("postgres", connStr)
	if dbErr != nil {
		log.Fatal(dbErr)
	}

	m, a := getDB()
	if len(m) != 0 || len(a) != 0 {
		sMg = m
		sA = a
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
					updateActDB(logActList())
					updateMemDB(logMemList())

					if actList() == nil {
						reply(event, linebot.NewTextMessage(userName+" "+str[1]+" 活動 : "+str[4]+" 時段 : "+str[2]+" "+str[3]))
					} else {
						reply(event, linebot.NewTextMessage(userName+" "+str[1]+" 活動 : "+str[4]+" 時段 : "+str[2]+" "+str[3]), actList())
					}

					return
				}

				for k, v := range sMg {
					if v.Member == userName && v.Date == str[0] && v.Clock == str[1] && v.Number == str[4] {
						if str[2] == "參加" {
							return
						} else if str[2] == "取消" {
							sMg = append(sMg[:k], sMg[k+1:]...)
							reply(event, linebot.NewTextMessage(userName+" "+str[2]+" 活動 : "+str[3]+" 時段 : "+str[0]+" "+str[1]), linebot.NewTextMessage(memList()))
							updateMemDB(logMemList())
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
				updateMemDB(logMemList())
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
					mString := logMemList()
					aString := logActList()
					reply(event, linebot.NewTextMessage(mString), linebot.NewTextMessage(aString))
				}

				if message.Text == "clearAll" {
					sMg = sMg[:0]
					sA = sA[:0]
					reply(event, linebot.NewTextMessage("Success clearAll"))
					updateActDB(logActList())
					updateMemDB(logMemList())
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
							schedule(sa[0] + " " + sa[1])
							updateActDB(logActList())
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
