package main

import (
	"fmt"
	"github.com/eaddingtonwhite/feed-the-gopher/internal/controllers"
	"github.com/go-co-op/gocron"
	"github.com/momentohq/client-sdk-go/auth"
	"github.com/momentohq/client-sdk-go/config"
	"github.com/momentohq/client-sdk-go/momento"
	"net/http"
	"time"
)

func task() {
	fmt.Println("I am running task.")
}

func main() {
	credProvider, err := auth.NewEnvMomentoTokenProvider("MOMENTO_AUTH_TOKEN")
	if err != nil {
		panic(err)
	}
	topicClient, err := momento.NewTopicClient(
		config.TopicsDefault(),
		credProvider,
	)
	if err != nil {
		panic(err)
	}
	chatController := &controllers.ChatController{
		MomentoTopicClient: topicClient,
	}

	cacheClient, err := momento.NewCacheClient(
		config.InRegionLatest(),
		credProvider,
		60*time.Second,
	)
	gameController := &controllers.GameController{
		MomentoClient: cacheClient,
	}

	http.HandleFunc("/connect", chatController.Connect)
	http.HandleFunc("/send-message", chatController.SendMessage)
	http.HandleFunc("/register-hit", gameController.Feed)
	http.HandleFunc("/top-scorers", gameController.GetTopFeeders)
	http.HandleFunc("/build-auto-feeder", gameController.BuildAutoFeeder)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})
	s := gocron.NewScheduler(time.UTC)
	_, err = s.Every(60).Seconds().Do(gameController.RunAutoFeederCalc)
	if err != nil {
		panic(err)
	}
	s.StartAsync()

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}

}
