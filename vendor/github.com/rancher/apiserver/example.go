package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/server"
	"github.com/rancher/apiserver/pkg/store/apiroot"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
)

type Foo struct {
	Bar string `json:"bar"`
}

type FooStore struct {
	empty.Store
}

func (f *FooStore) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return types.APIObject{
		Type: "foos",
		ID:   id,
		Object: Foo{
			Bar: "baz",
		},
	}, nil
}

func (f *FooStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	return types.APIObjectList{
		Objects: []types.APIObject{
			{
				Type: "foostore",
				ID:   "foo",
				Object: Foo{
					Bar: "baz",
				},
			},
		},
	}, nil
}

func main() {
	// Create the default server
	s := server.DefaultAPIServer()

	// Add some types to it and setup the store and supported methods
	s.Schemas.MustImportAndCustomize(Foo{}, func(schema *types.APISchema) {
		schema.Store = &FooStore{}
		schema.CollectionMethods = []string{http.MethodGet}
	})

	// Register root handler to list api versions
	apiroot.Register(s.Schemas, []string{"v1", "v2"})

	// Setup mux router to assign variables the server will look for (refer to MuxURLParser for all variable names)
	router := mux.NewRouter()
	router.Handle("/{prefix}/{type}", s)
	router.Handle("/{prefix}/{type}/{id}", s)

	// When a route is found construct a custom API request to serves up the API root content
	router.NotFoundHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s.Handle(&types.APIRequest{
			Request:   r,
			Response:  rw,
			Type:      "apiRoot",
			URLPrefix: "v1",
		})
	})

	// Start API Server
	log.Print("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
