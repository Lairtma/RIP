package api

import (
	"RIP/internal/app/config"
	"RIP/internal/app/ds"
	"RIP/internal/app/dsn"
	"RIP/internal/app/repository"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Application struct {
	repo   *repository.Repository
	config *config.Config
}

type OrderShowText struct {
	Text    ds.TextToEncOrDec
	EncType string
	Res     string
}

func GetUserId() int {
	return 1
}
func GetModaratorId() int {
	return 2
}

func New() (*Application, error) {
	var err error
	app := Application{}
	app.config, err = config.NewConfig()
	if err != nil {
		return nil, err
	}

	app.repo, err = repository.New(dsn.FromEnv())
	if err != nil {
		return nil, err
	}

	return &app, nil
}

func (a *Application) Run() {
	log.Println("Server start up")

	r := gin.Default()
	var err error
	r.SetFuncMap(template.FuncMap{
		"replaceNewline": func(text string) template.HTML {
			return template.HTML(strings.ReplaceAll(text, "/n", "<br>"))
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
	})

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.LoadHTMLGlob("templates/*")
	r.Static("/css", "./resources/css")

	r.GET("/textsencordec", func(c *gin.Context) {
		encordec := c.Query("query") // Получаем поисковый запрос из URL

		var FiltredTexts []ds.TextToEncOrDec

		if encordec == "" {
			FiltredTexts, err = a.repo.GetAllTexts()
			if err != nil {
				log.Println("unable to get all texts")
				c.Error(err)
				return
			}
		} else {
			var encType bool
			if encordec == "en" {
				encType = true
			} else {
				encType = false
			}
			FiltredTexts, err = a.repo.GetTextByType(encType)
			if err != nil {
				log.Println("unable to get text by type")
				c.Error(err)
				return
			}
		}

		var order_len int
		var order_Id int
		order_wrk, err := a.repo.GetWorkingOrderByUserId(GetUserId())
		log.Println(order_wrk)
		if err != nil {
			log.Println("unable to get working order")
		}
		if len(order_wrk) == 0 {
			order_len = 0
			order_Id = 0

		} else {
			milkmeals_in_wrk_req, err := a.repo.GetTextIdsByOrderId(order_wrk[0].Id)
			if err != nil {
				log.Println("unable to get text ids by order")
			}
			order_len = len(milkmeals_in_wrk_req)
			order_Id = order_wrk[0].Id
		}

		c.HTML(http.StatusOK, "textsencordec.html", gin.H{
			"title":     "Main website",
			"first_row": FiltredTexts,
			"query":     encordec,
			"len":       order_len,
			"order_id":  order_Id,
		})
	})

	r.POST("/textsencordec", func(c *gin.Context) {

		id := c.PostForm("add")
		log.Println(id)

		textId, err := strconv.Atoi(id)

		text, err := a.repo.GetTextByID(textId)
		var encType string
		if text.Enc {
			encType = "Тип:/nШифрование с битом чётности"
		} else {
			encType = "Тип:/nДешифрование с битом чётности"
		}

		if err != nil { // если не получилось
			log.Printf("cant transform ind", err)
			c.Error(err)
			c.String(http.StatusBadRequest, "Invalid ID")
			return
		}

		order_work, err := a.repo.GetWorkingOrderByUserId(GetUserId())
		var order_ID int
		if len(order_work) == 0 {
			new_order, err := a.repo.CreateOrder(GetUserId(), GetModaratorId())
			if err != nil {
				log.Println("unable to create milk request")
			}
			order_ID = new_order[0].Id
		} else {
			order_ID = order_work[0].Id
		}
		position, err := a.repo.GetTextIdsByOrderId(order_ID)
		a.repo.AddToOrder(order_ID, textId, len(position)+1, encType)

		c.Redirect(301, "/textsencordec")

	})

	r.GET("/encordecorder/:id", func(c *gin.Context) {
		id := c.Param("id")
		index, err := strconv.Atoi(id)
		if err != nil { // если не получилось
			c.String(http.StatusBadRequest, "Invalid ID")
			return
		}

		order, err := a.repo.GetOrderByID(index)
		if err != nil {
			log.Printf("cant get cart by id %v", err)
		}
		if order.Status == 3 {
			c.Redirect(301, "/textsencordec")
		}

		TextIDs, err := a.repo.GetTextsByOrderId(index)
		if err != nil {
			log.Println("unable to get MealsIDsByCartID")
			c.Error(err)
			return
		}

		TextsInCart := []OrderShowText{}
		for _, v := range TextIDs {
			text_tmp, err := a.repo.GetTextByID(v.TextID)
			if err != nil {
				c.Error(err)
				return
			}
			TextsInCart = append(TextsInCart, OrderShowText{Text: *text_tmp, EncType: v.EncType, Res: v.Res})
		}
		userFio, err := a.repo.GetUserFioById(order.CreatorID)
		if err != nil {
			c.Error(err)
			return
		}
		c.HTML(http.StatusOK, "encordecorder.html", gin.H{
			"title":     "Main website",
			"orderId":   index,
			"card_data": TextsInCart,
			"user":      userFio,
		})
	})

	r.POST("/encordecorder/:id", func(c *gin.Context) {

		id := c.Param("id")
		index, err := strconv.Atoi(id)
		if err != nil { // если не получилось
			log.Printf("cant get cart by id %v", err)
			c.Error(err)
			c.String(http.StatusBadRequest, "Invalid ID")
			return
		}

		err = a.repo.DeleteOrder(index)
		if err != nil { // если не получилось
			log.Printf("cant delete cart by id %v", err)
			c.Error(err)
			c.String(http.StatusBadRequest, "Invalid ID")
			return
		}

		c.Redirect(301, "/textsencordec")

	})

	r.GET("/text/:id", func(c *gin.Context) {
		id := c.Param("id") // Получаем ID из URL
		index, err := strconv.Atoi(id)

		if err != nil { // если не получилось
			log.Printf("cant get card by id %v", err)
			c.Error(err)
			c.String(http.StatusBadRequest, "Invalid ID")
			return
		}

		text, err := a.repo.GetTextByID(index)
		if err != nil { // если не получилось
			log.Printf("cant get card by id %v", err)
			c.Error(err)
			c.String(http.StatusBadRequest, "Invalid ID")
			return
		}
		c.HTML(http.StatusOK, "text.html", gin.H{
			"title":     "Main website",
			"card_data": text,
		})
	})

	r.Static("/image", "./resources")

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")

	log.Println("Server down")
}
