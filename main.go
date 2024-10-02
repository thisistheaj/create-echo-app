package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"your_project_name/templates"
	"your_project_name/types"
)

var db *gorm.DB

//
// ----- 1. INIT & ROUTER -----
//

func main() {

	// server & session setup
	e := echo.New()
	sessionSecret := os.Getenv("SESSION_SECRET")
	store := sessions.NewCookieStore([]byte(sessionSecret))
	e.Use(session.Middleware(store))

	var err error
	db, err = initializeDatabase()
	if err != nil {
		log.Fatal(err)
	}

	// routes
	e.GET("/", homePage)
	e.GET("/register", showRegisterForm)
	e.POST("/register", register)
	e.GET("/login", showLoginForm)
	e.POST("/login", login)
	e.GET("/logout", logout)
	e.GET("/dashboard", dashboard, requireAuth)
	e.GET("/posts/create", showCreatePostForm, requireAuth)
	e.POST("/posts", createPost, requireAuth)
	e.GET("/posts/:id", showPost)
	e.GET("/posts/:id/edit", showEditPostForm, requireAuth)
	e.POST("/posts/:id/delete", deletePost, requireAuth)
	e.POST("/posts/:id/edit", updatePost, requireAuth)
	e.Static("/public", "public")

	// start server
	port := os.Getenv("PORT")
	if os.Getenv("APP_ENV") == "dev" {
		port = "8080"
	}
	fmt.Println("Server is running on http://localhost:" + port)
	if err := e.Start(":" + port); err != nil {
		log.Fatal(err)
	}
}

//
// ----- 2. MIDDLEWARE -----
//

func requireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, _ := session.Get("session", c)
		if auth, ok := sess.Values["authenticated"].(bool); !ok || !auth {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		return next(c)
	}
}

//
// 3. HELPER FUNCTIONS
//

func isAuthenticated(c echo.Context) bool {
	sess, _ := session.Get("session", c)
	if auth, ok := sess.Values["authenticated"].(bool); ok && auth {
		return true
	}
	return false
}

func initializeDatabase() (*gorm.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	databasePublicURL := os.Getenv("DATABASE_PUBLIC_URL")

	var dbConnectionString string
	if os.Getenv("APP_ENV") == "dev" {
		dbConnectionString = databasePublicURL
	} else {
		dbConnectionString = databaseURL
	}

	db, err := gorm.Open(postgres.Open(dbConnectionString), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	err = db.AutoMigrate(&types.User{}, &types.Post{})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	return db, nil
}

//
// ----- 4. CONTROLLERS -----
//

func homePage(c echo.Context) error {
	var posts []types.Post
	db.Order("created_at DESC").Limit(5).Find(&posts)
	isAuth := isAuthenticated(c)
	return templates.Home(posts, isAuth).Render(c.Request().Context(), c.Response().Writer)
}

func showRegisterForm(c echo.Context) error {
	isAuth := isAuthenticated(c)
	return templates.RegisterForm(isAuth).Render(c.Request().Context(), c.Response().Writer)
}

func register(c echo.Context) error {
	email := c.FormValue("email")
	password := c.FormValue("password")
	isAuth := isAuthenticated(c)

	var existingUser types.User
	result := db.Where("email = ?", email).First(&existingUser)
	if result.RowsAffected > 0 {
		return templates.Error("User already exists", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return templates.Error("Error hashing password", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	newUser := types.User{
		Email:    email,
		Password: string(hashedPassword),
	}

	if err := db.Create(&newUser).Error; err != nil {
		return templates.Error("Error creating user", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	return c.Redirect(http.StatusSeeOther, "/login")
}

func showLoginForm(c echo.Context) error {
	isAuth := isAuthenticated(c)
	return templates.LoginForm(isAuth).Render(c.Request().Context(), c.Response().Writer)
}

func login(c echo.Context) error {
	email := c.FormValue("email")
	password := c.FormValue("password")
	isAuth := isAuthenticated(c)

	var user types.User
	result := db.Where("email = ?", email).First(&user)
	if result.RowsAffected == 0 {
		return templates.Error("Invalid credentials", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return templates.Error("Invalid credentials", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	sess, _ := session.Get("session", c)
	sess.Values["authenticated"] = true
	sess.Values["email"] = email
	sess.Values["user_id"] = user.ID
	sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 1 week
		HttpOnly: true,
	}
	sess.Save(c.Request(), c.Response())

	return c.Redirect(http.StatusSeeOther, "/dashboard")
}

func logout(c echo.Context) error {
	sess, _ := session.Get("session", c)
	sess.Values["authenticated"] = false
	sess.Values["email"] = ""
	sess.Values["user_id"] = nil
	sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	sess.Save(c.Request(), c.Response())

	return c.Redirect(http.StatusSeeOther, "/")
}

func dashboard(c echo.Context) error {
	sess, _ := session.Get("session", c)
	email := sess.Values["email"].(string)
	userID := sess.Values["user_id"].(uint)

	var posts []types.Post
	db.Where("user_id = ?", userID).Find(&posts)

	return templates.Dashboard(email, posts).Render(c.Request().Context(), c.Response().Writer)
}

func showCreatePostForm(c echo.Context) error {
	return templates.CreatePostForm().Render(c.Request().Context(), c.Response().Writer)
}

func createPost(c echo.Context) error {
	sess, _ := session.Get("session", c)
	userID := sess.Values["user_id"].(uint)
	isAuth := isAuthenticated(c)

	title := c.FormValue("title")
	body := c.FormValue("body")

	// Handle file upload
	file, err := c.FormFile("image")
	var imageURL string
	if err == nil {
		// Create the user-specific uploads directory if it doesn't exist
		uploadDir := fmt.Sprintf("public/uploads/user/%d", userID)
		if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
			return templates.Error("Error creating uploads directory", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}

		// Open the uploaded file
		src, err := file.Open()
		if err != nil {
			return templates.Error("Error opening uploaded file", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}
		defer src.Close()

		// Generate a unique filename
		filename := filepath.Join(uploadDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename))

		// Create the destination file
		dst, err := os.Create(filename)
		if err != nil {
			return templates.Error("Error creating destination file", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}
		defer dst.Close()

		// Copy the uploaded file to the destination file
		if _, err = io.Copy(dst, src); err != nil {
			return templates.Error("Error saving uploaded file", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}

		// Set the image URL to the relative path
		imageURL = "/" + filename
	}

	post := types.Post{
		Title:    title,
		Body:     body,
		ImageURL: imageURL,
		UserID:   int(userID),
	}

	if err := db.Create(&post).Error; err != nil {
		return templates.Error("Error creating post", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	return c.Redirect(http.StatusSeeOther, "/dashboard")
}

func showPost(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var post types.Post
	isAuth := isAuthenticated(c)
	if err := db.First(&post, id).Error; err != nil {
		return templates.Error("Post not found", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	return templates.ShowPost(post, isAuth).Render(c.Request().Context(), c.Response().Writer)
}

func showEditPostForm(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var post types.Post
	isAuth := isAuthenticated(c)
	if err := db.First(&post, id).Error; err != nil {
		return templates.Error("Post not found", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	sess, _ := session.Get("session", c)
	userID := sess.Values["user_id"].(uint)

	if post.UserID != int(userID) {
		return templates.Error("You don't have permission to edit this post", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	return templates.EditPostForm(post).Render(c.Request().Context(), c.Response().Writer)
}

func updatePost(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var post types.Post
	isAuth := isAuthenticated(c)
	if err := db.First(&post, id).Error; err != nil {
		return templates.Error("Post not found", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	sess, _ := session.Get("session", c)
	userID := sess.Values["user_id"].(uint)

	if post.UserID != int(userID) {
		return templates.Error("You don't have permission to edit this post", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	post.Title = c.FormValue("title")
	post.Body = c.FormValue("body")

	// Handle file upload
	file, err := c.FormFile("image")
	if err == nil {
		// Create the user-specific uploads directory if it doesn't exist
		uploadDir := fmt.Sprintf("public/uploads/user/%d", userID)
		if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
			return templates.Error("Error creating uploads directory", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}

		// Open the uploaded file
		src, err := file.Open()
		if err != nil {
			return templates.Error("Error opening uploaded file", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}
		defer src.Close()

		// Generate a unique filename
		filename := filepath.Join(uploadDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename))

		// Create the destination file
		dst, err := os.Create(filename)
		if err != nil {
			return templates.Error("Error creating destination file", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}
		defer dst.Close()

		// Copy the uploaded file to the destination file
		if _, err = io.Copy(dst, src); err != nil {
			return templates.Error("Error saving uploaded file", isAuth).Render(c.Request().Context(), c.Response().Writer)
		}

		// Set the new image URL
		post.ImageURL = "/" + filename
	}

	if err := db.Save(&post).Error; err != nil {
		return templates.Error("Error updating post", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/posts/%d", post.ID))
}

func deletePost(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var post types.Post
	isAuth := isAuthenticated(c)
	if err := db.First(&post, id).Error; err != nil {
		return templates.Error("Post not found", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	if err := db.Delete(&post).Error; err != nil {
		return templates.Error("Error deleting post", isAuth).Render(c.Request().Context(), c.Response().Writer)
	}

	return c.Redirect(http.StatusSeeOther, "/dashboard")
}
