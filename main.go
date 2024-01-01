package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var rnd *renderer.Render
var client *mongo.Client
var db *mongo.Database

type (
	TodoModel struct {
		Id        primitive.ObjectID `bson:"id,omitempty"`
		Title     string             `bson:"title,omitempty"`
		Completed bool               `bson:"completed,omitempty"`
		CreatedAt time.Time          `bson:"completed_at,omitempty"`
	}

	Todo struct {
		Id        string    `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"completed_at"`
	}

	GetTodoResponse struct {
		Message string `json:"message"`
		Data    []Todo `json:"data"`
	}
)

func init() {
	fmt.Println("Running the init function")
	rnd = renderer.New()

	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	checkError(err)

	err = client.Ping(ctx, readpref.Primary())
	checkError(err)

	db = client.Database(dbName)
}

func checkError(err error) {
	if err != nil {
		fmt.Printf("Error encountered : %+v\n", err)
	}
}

func main() {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Get("/", homeHandler)
	router.Mount("/todo", todoHandlers())

	server := &http.Server{
		Addr:         ":9000",
		Handler:      chi.NewRouter(),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	go func() {
		fmt.Println("Server started on port ", 9000)
		if err := server.ListenAndServe(); err != nil {
			fmt.Printf("listen:%s\n", err)
		}
	}()

	sig := <-stopChan
	fmt.Printf("signal received : %+v\n", sig)

	if err := client.Disconnect(context.Background()); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Printf("Server shutdown failed : %v\n", err)
	}
	fmt.Printf("Server shutdown gracefully")
}

func homeHandler(rw http.ResponseWriter, r *http.Request) {
	filePath := "./README.md"
	err := rnd.FileView(rw, http.StatusOK, filePath, "readme.md")
	checkError(err)
}

func todoHandlers() http.Handler {
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Get("/", getTodos)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return router
}

func getTodos(rw http.ResponseWriter, r *http.Request) {
	var todoListFromDB = []TodoModel{}
	filter := bson.D{}
	cursor, err := db.Collection(collectionName).Find(context.Background(), filter)

	if err != nil {
		fmt.Printf("Failed to fetch todos from db records : %+v", err.Error())
		rnd.JSON(rw, http.StatusBadRequest, renderer.M{
			"message": "Could not fetch the todo collection",
			"error":   err.Error(),
		})
		return
	}

	todoList := []Todo{}

	if err := cursor.All(context.Background(), &todoListFromDB); err != nil {
		checkError(err)
	}

	for _, td := range todoListFromDB {
		todoList = append(todoList, Todo{
			Id:        td.Id.Hex(),
			Title:     td.Title,
			Completed: td.Completed,
			CreatedAt: td.CreatedAt,
		})
	}

	rnd.JSON(rw, http.StatusOK, GetTodoResponse{
		Message: "All Todos retrieved",
		Data:    todoList,
	})

}
