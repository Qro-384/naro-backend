package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

type City struct {
	ID          int            `json:"id,omitempty"  db:"ID"`
	Name        sql.NullString `json:"name,omitempty"  db:"Name"`
	CountryCode sql.NullString `json:"countryCode,omitempty"  db:"CountryCode"`
	District    sql.NullString `json:"district,omitempty"  db:"District"`
	Population  sql.NullInt64  `json:"population,omitempty"  db:"Population"`
}

func getCityInfoHandler(c echo.Context) error {
	cityName := c.Param("cityName")
	fmt.Println(cityName)

	var city City
	db.Get(&city, "SELECT * FROM city WHERE Name=?", cityName)
	if !city.Name.Valid {
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, city)
}

type countryName struct {
	Name string `json:"name" db:"Name"`
}

type cityName struct {
	Name string `json:"name" db:"Name"`
}

func getCountryCityListHandler(c echo.Context) error {
	countryName := c.Param("countryName")
	var countryCode string
	db.Get(&countryCode, "SELECT Code FROM country WHERE Name=?", countryName)
	if countryCode == "" {
		return c.NoContent(http.StatusNotFound)
	}

	var cities []cityName
	var cityNumber int
	db.Get(&cityNumber, "SELECT COUNT(*) FROM city WHERE countryCode=?", countryCode)

	for i := 0; i < cityNumber; i++ {
		var oneCity cityName
		db.Get(&oneCity, "SELECT Name FROM city WHERE countryCode=? LIMIT 1 OFFSET ?", countryCode, i)
		if oneCity.Name == "" {
			return c.NoContent(http.StatusNotFound)
		}

		cities = append(cities, oneCity)
	}
	return c.JSON(http.StatusOK, cities)

}

func getCountryInfoAllHandler(c echo.Context) error {
	var countryNumber int
	db.Get(&countryNumber, "SELECT COUNT(*) FROM country")
	fmt.Println(countryNumber)
	if countryNumber == 0 {
		return c.NoContent(http.StatusNotFound)
	}

	var countries []countryName
	for i := 0; i < countryNumber; i++ {
		var oneCountry countryName
		db.Get(&oneCountry, "SELECT Name FROM country LIMIT 1 OFFSET ?", i)
		if oneCountry.Name == "" {
			return c.NoContent(http.StatusNotFound)
		}

		countries = append(countries, oneCountry)
	}

	return c.JSON(http.StatusOK, countries)

}

func testHandler(c echo.Context) error {
	var oneCity City
	db.Get(&oneCity, "SELECT * FROM city LIMIT 1")
	return c.JSON(http.StatusOK, oneCity)
}

func postCityHandler(c echo.Context) error {
	var city City
	err := c.Bind(&city)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request body")
	}

	result, err := db.Exec("INSERT INTO city (Name, CountryCode, District, Population) VALUES (?, ?, ?, ?)", city.Name, city.CountryCode, city.District, city.Population)
	if err != nil {
		log.Printf("failed to insert city data: %s\n", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	id, err := result.LastInsertId()
	if err != nil {
		fmt.Printf("failed to get last insert id: %s\n", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	city.ID = int(id)

	return c.JSON(http.StatusCreated, city)
}

type LoginRequestBody struct {
	Username string `json:"username,omitempty" form:"username"`
	Password string `json:"password,omitempty" form:"password"`
}

func signUpHandler(c echo.Context) error {
	// リクエストを受け取り、reqに格納する
	req := LoginRequestBody{}
	c.Bind(&req)

	// バリデーションする(PasswordかUsernameが空文字列の場合は400 BadRequestを返す)
	if req.Password == "" || req.Username == "" {
		return c.String(http.StatusBadRequest, "Username or Password is empty")
	}

	// 登録しようとしているユーザーが既にデータベース内に存在するかチェック
	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM users WHERE Username=?", req.Username)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	// 存在したら409 Conflictを返す
	if count > 0 {
		return c.String(http.StatusConflict, "Username is already used")
	}

	// パスワードをハッシュ化する
	pw := req.Password + salt
	hashedPass, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	// ハッシュ化に失敗したら500 InternalServerErrorを返す
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// ユーザーを登録する
	_, err = db.Exec("INSERT INTO users (Username, HashedPass) VALUES (?, ?)", req.Username, hashedPass)
	// 登録に失敗したら500 InternalServerErrorを返す
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	// 登録に成功したら201 Createdを返す
	return c.NoContent(http.StatusCreated)
}

type User struct {
	Username   string `json:"username,omitempty" form:"Username"`
	HashedPass string `json:"-" form:"HashedPass"`
}

func loginHandler(c echo.Context) error {
	var req LoginRequestBody
	c.Bind(&req)

	if req.Password == "" || req.Username == "" {
		return c.String(http.StatusBadRequest, "Username or Password is empty")
	}

	log.Println(req.Username)
	user := User{}
	username := req.Username
	err := db.QueryRow("SELECT * FROM users WHERE Username=?", username).Scan(&user.Username, &user.HashedPass)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.NoContent(http.StatusUnauthorized)
		} else {
			log.Println(err)
			log.Println(1)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.HashedPass), []byte(req.Password+salt))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return c.NoContent(http.StatusUnauthorized)
		} else {
			log.Println(2)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	sess, err := session.Get("sessions", c)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "something wrong in getting session")
	}
	sess.Values["userName"] = req.Username
	sess.Save(c.Request(), c.Response())

	return c.NoContent(http.StatusOK)

}

func userAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get("sessions", c)
		if err != nil {
			fmt.Println(err)
			return c.String(http.StatusInternalServerError, "something wrong in getting session")
		}
		if sess.Values["userName"] == nil {
			return c.String(http.StatusUnauthorized, "please login")
		}
		c.Set("userName", sess.Values["userName"].(string))

		return next(c)
	}
}

type Me struct {
	Username string `json:"username,omitempty"  db:"username"`
}

func getWhoAmIHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, Me{
		Username: c.Get("userName").(string),
	})
}
