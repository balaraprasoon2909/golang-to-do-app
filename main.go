package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi"
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

	CreateTodo struct {
		Title string `json:"title"`
	}

	UpdateTodo struct {
		Title     string `json:"title"`
		Completed bool   `json:"completed"`
	}
)

func init() {
	log.Println("Running the init function")
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
		log.Printf("Error encountered : %+v\n", err)
	}
}

func main() {
	router := chi.NewRouter()
	log.Printf("ascnllk")
	// router.Use(middleware.Logger)
	router.Get("/", homeHandler)
	router.Mount("/todo", todoHandlers())

	server := &http.Server{
		Addr:         ":9000",
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	go func() {
		log.Println("Server started on port ", 9000)
		if err := server.ListenAndServe(); err != nil {
			log.Printf("listen:%s\n", err)
		}
	}()

	sig := <-stopChan
	log.Printf("signal received : %+v\n", sig)

	if err := client.Disconnect(context.Background()); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown failed : %v\n", err)
	}
	log.Printf("Server shutdown gracefully")
}

func homeHandler(rw http.ResponseWriter, r *http.Request) {
	log.Printf("inside home handler")
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
		log.Printf("Failed to fetch todos from db records : %+v\n", err.Error())
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

func createTodo(rw http.ResponseWriter, r *http.Request) {
	var request CreateTodo

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Failed to decode json data : %+v", err.Error())
		rnd.JSON(rw, http.StatusBadRequest, renderer.M{
			"message": "Could not decode data",
		})
		return
	}

	if len(request.Title) == 0 {
		log.Printf("No title added to response body")
		rnd.JSON(rw, http.StatusBadRequest, renderer.M{
			"message": "Please add a title",
		})
		return
	}

	todoModel := TodoModel{
		Id:        primitive.NewObjectID(),
		Title:     request.Title,
		Completed: false,
		CreatedAt: time.Now(),
	}

	data, err := db.Collection(collectionName).InsertOne(r.Context(), todoModel)
	if err != nil {
		log.Printf("Failed to insert data into the database : %+v\n", err.Error())
		rnd.JSON(rw, http.StatusInternalServerError, renderer.M{
			"message": "Failed to add data into the database",
			"error":   err.Error(),
		})
	}

	rnd.JSON(rw, http.StatusCreated, renderer.M{
		"message": "Todo created successfully",
		"ID":      data.InsertedID,
	})
}

func deleteTodo(rw http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		log.Printf("Invalid id : %+v\n", err.Error())
		rnd.JSON(rw, http.StatusBadRequest, err.Error())
		return
	}

	filter := bson.M{"id": res}
	if data, err := db.Collection(collectionName).DeleteOne(r.Context(), filter); err != nil {
		log.Printf("Could not delete item from database : %+v\n", err.Error())
		rnd.JSON(rw, http.StatusInternalServerError, renderer.M{
			"message": "An error occurred while deleting the todo item",
			"error":   err.Error(),
		})
	} else {
		rnd.JSON(rw, http.StatusOK, renderer.M{
			"message": "Item deleted successfully",
			"data":    data,
		})
	}
}

func updateTodo(rw http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	res, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Printf("Invalid id : %+v\n", err.Error())
		rnd.JSON(rw, http.StatusBadRequest, err.Error())
		return
	}

	var request UpdateTodo

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Failed to decode json response body data: %+v\n", err.Error())
		rnd.JSON(rw, http.StatusBadRequest, err.Error())
	}

	if len(request.Title) == 0 {
		rnd.JSON(rw, http.StatusBadRequest, renderer.M{
			"message": "Title cannot be empty",
		})
		return
	}

	filter := bson.M{"id": res}
	update := bson.M{"$set": bson.M{"title": request.Title, "completed": request.Completed}}

	data, err := db.Collection(collectionName).UpdateOne(r.Context(), filter, update)

	if err != nil {
		log.Printf("Failed to update db collection : %+v\n", err.Error())
		rnd.JSON(rw, http.StatusInternalServerError, renderer.M{
			"message": "Failed to update data in db collection",
			"error":   err.Error(),
		})
		return
	}

	rnd.JSON(rw, http.StatusOK, renderer.M{
		"message": "Successfully updated item",
		"data":    data.ModifiedCount,
	})
}
