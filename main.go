package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dgraph-io/dgo/v200"
	"github.com/dgraph-io/dgo/v200/protos/api"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"google.golang.org/grpc"
)

type drawflow struct {
	Uid    string `json:"uid,omitempty"`
	Name   string `json:"name,omitempty"`
	Editor string `json:"editor,omitempty"`
}
type CancelFunc func()

func getDgraphClient() (*dgo.Dgraph, CancelFunc) {
	conn, err := grpc.Dial("localhost:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}

	dc := api.NewDgraphClient(conn)
	dg := dgo.NewDgraphClient(dc)

	return dg, func() {
		if err := conn.Close(); err != nil {
			log.Printf("Error while closing connection:%v", err)
		}
	}
}

func main() {
	dg, cancel := getDgraphClient()
	defer cancel()
	ctx := context.Background()
	initDatabase(ctx, dg)

	fmt.Println("Server running...")
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-XSRF-TOKEN", "XSRF-TOKEN"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	router.Use(cors.Handler)
	router.Get("/saveEditor", saveEditor(ctx, dg))
	router.Get("/readEditor", readEditor(ctx, dg))

	fmt.Println("Server listening on port: 3000")
	log.Fatal(http.ListenAndServe(":3000", router))

}

func initDatabase(ctx context.Context, dg *dgo.Dgraph) {
	dg.Alter(context.Background(), &api.Operation{DropOp: api.Operation_ALL})

	op := &api.Operation{}
	op.Schema = `
					name: string @index(exact, term) @lang .
					editor: string .
				`
	dg.Alter(ctx, op)
}

func saveEditor(ctx context.Context, dg *dgo.Dgraph) func(http.ResponseWriter, *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		var new_editor = ""
		q := `
		{
			me(func: eq(name, "Editor")) {
					uid
			}
		}`

		resp, _ := dg.NewTxn().Query(ctx, q)

		resp_str := string(resp.Json)

		if strings.Contains(resp_str, "uid") {
			type Root struct {
				Me []drawflow `json:"me"`
			}

			var r Root
			json.Unmarshal(resp.GetJson(), &r)

			request.ParseForm()
			Editor, isEditor := request.Form["Editor"]
			if isEditor {
				new_editor = Editor[0]
			} else {
				json.NewEncoder(response).Encode("There is a editor but no a new one")
				return
			}

			fmt.Println(r.Me[0].Uid)

			drawflowUpdate := drawflow{
				Uid:    r.Me[0].Uid,
				Name:   "Editor",
				Editor: new_editor,
			}

			pb, _ := json.Marshal(drawflowUpdate)

			mu := &api.Mutation{
				CommitNow: true,
			}

			mu.SetJson = pb
			_, err := dg.NewTxn().Mutate(ctx, mu)
			if err != nil {
				log.Fatal(err)
			}
			json.NewEncoder(response).Encode("updated")

		} else {
			request.ParseForm()
			Editor, isEditor := request.Form["Editor"]
			if isEditor {
				new_editor = Editor[0]
			} else {
				json.NewEncoder(response).Encode("there is no a editor")
				return
			}
			drawflowCreate := drawflow{
				Uid:    "_:editor",
				Name:   "Editor",
				Editor: new_editor,
			}

			pb, _ := json.Marshal(drawflowCreate)

			mu := &api.Mutation{
				CommitNow: true,
			}

			mu.SetJson = pb
			dg.NewTxn().Mutate(ctx, mu)
			json.NewEncoder(response).Encode("created")
		}

	}
}

func readEditor(ctx context.Context, dg *dgo.Dgraph) func(http.ResponseWriter, *http.Request) {
	return func(response http.ResponseWriter, req *http.Request) {
		q := `
		{
			me(func: eq(name, "Editor")) {
					uid
					editor
			}
		}`

		resp, _ := dg.NewTxn().Query(ctx, q)

		resp_str := string(resp.Json)

		if strings.Contains(resp_str, "uid") {
			type Root struct {
				Me []drawflow `json:"me"`
			}

			var r Root
			json.Unmarshal(resp.GetJson(), &r)

			json.NewEncoder(response).Encode(r.Me[0].Editor)
		} else {
			json.NewEncoder(response).Encode("Empty")
		}

	}
}
