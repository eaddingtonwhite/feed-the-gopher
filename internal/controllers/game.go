package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	serviceconfig "github.com/eaddingtonwhite/feed-the-gopher/internal/config"

	"github.com/google/uuid"
	"github.com/momentohq/client-sdk-go/momento"
	"github.com/momentohq/client-sdk-go/responses"
	"github.com/momentohq/client-sdk-go/utils"
)

var leaderboardFetchCount uint32 = 100

type GameController struct {
	MomentoClient momento.CacheClient
}

type buttonHitRequest struct {
	User string `json:"user"`
}

type autoFeederBuildRequest struct {
	User string `json:"user"`
	Type int    `json:"type"`
}

type scoreBoardEntry struct {
	Rank  int     `json:"rank"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}
type scoreBoardResponse struct {
	Elements []scoreBoardEntry `json:"elements"`
}

func (c *GameController) feedRateLimit(ctx context.Context, w http.ResponseWriter, userID string) bool {
	rateLimitRsp, err := c.MomentoClient.DictionaryIncrement(ctx, &momento.DictionaryIncrementRequest{
		CacheName:      serviceconfig.CacheName,
		DictionaryName: userID + "-rate-limit",
		Field:          momento.String("/game/feed"),
		Amount:         1,
		Ttl: &utils.CollectionTtl{
			Ttl:        60 * time.Second,
			RefreshTtl: false,
		},
	})
	if err != nil {
		writeFatalError(w, "error checking rate limit", err)
	}
	switch r := rateLimitRsp.(type) {
	case *responses.DictionaryIncrementSuccess:
		if r.Value() > serviceconfig.MaxManualFeedRatePerMinute {
			writeError(http.StatusTooManyRequests, "rate-limit exceeded", w)
			return true
		}
	}
	return false
}

func (c *GameController) Feed(w http.ResponseWriter, r *http.Request) {
	var request buttonHitRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeFatalError(w, "fatal error occurred decoding msg payload", err)
	}

	if c.feedRateLimit(r.Context(), w, request.User) {
		return
	}

	_, err := c.MomentoClient.SortedSetIncrementScore(r.Context(), &momento.SortedSetIncrementScoreRequest{
		CacheName: serviceconfig.CacheName,
		SetName:   "score-board",
		Value:     momento.String(request.User),
		Amount:    1,
		Ttl: &utils.CollectionTtl{
			Ttl:        24 * time.Hour,
			RefreshTtl: true,
		},
	})
	if err != nil {
		writeFatalError(w, "fatal error occurred incrementing user score", err)
	}
}

func (c *GameController) GetTopFeeders(w http.ResponseWriter, r *http.Request) {
	resp, err := c.MomentoClient.SortedSetFetchByScore(r.Context(), &momento.SortedSetFetchByScoreRequest{
		CacheName: serviceconfig.CacheName,
		SetName:   "score-board",
		Order:     momento.DESCENDING,
		Count:     &leaderboardFetchCount,
	})
	if err != nil {
		writeFatalError(w, "fatal error occurred getting top scores", err)
	}
	var scoreBoardEntries []scoreBoardEntry
	switch r := resp.(type) {
	case *responses.SortedSetFetchHit:
		for rank, e := range r.ValueStringElements() {
			scoreBoardEntries = append(scoreBoardEntries, scoreBoardEntry{
				Name:  e.Value,
				Value: e.Score,
				Rank:  rank + 1,
			})
		}
	}

	if err := json.NewEncoder(w).Encode(&scoreBoardResponse{Elements: scoreBoardEntries}); err != nil {
		writeFatalError(w, "fatal error getting score", err)
		return
	}
}

func (c *GameController) BuildAutoFeeder(w http.ResponseWriter, r *http.Request) {
	var request autoFeederBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeFatalError(w, "fatal error occurred decoding build feeder payload", err)
	}

	rsp, err := c.MomentoClient.SortedSetGetScore(r.Context(), &momento.SortedSetGetScoreRequest{
		CacheName: serviceconfig.CacheName,
		SetName:   "score-board",
		Value:     momento.String(request.User),
	})
	if err != nil {
		writeFatalError(w, "fatal error occurred fetching score board", err)
	}
	switch sr := rsp.(type) {
	case *responses.SortedSetGetScoreHit:
		if float64(serviceconfig.AutoFeeders[request.Type].Cost) < sr.Score() {
			// Purchase auto feeder remove cost from total score
			_, err := c.MomentoClient.DictionaryIncrement(r.Context(), &momento.DictionaryIncrementRequest{
				CacheName:      serviceconfig.CacheName,
				DictionaryName: request.User + "score-board",
				Field:          momento.String(strconv.Itoa(request.Type)),
				Amount:         int64(-serviceconfig.AutoFeeders[request.Type].Cost),
				Ttl: &utils.CollectionTtl{
					Ttl:        24 * time.Hour,
					RefreshTtl: true,
				},
			})
			if err != nil {
				writeFatalError(w, "fatal error occurred buying auto feeder", err)
			}
			// Purchase auto feeder increment player auto feeder count
			_, err = c.MomentoClient.DictionaryIncrement(r.Context(), &momento.DictionaryIncrementRequest{
				CacheName:      serviceconfig.CacheName,
				DictionaryName: request.User + "-auto-feeders",
				Field:          momento.String(strconv.Itoa(request.Type)),
				Amount:         1,
				Ttl: &utils.CollectionTtl{
					Ttl:        24 * time.Hour,
					RefreshTtl: true,
				},
			})
			if err != nil {
				writeFatalError(w, "fatal error occurred buying auto feeder", err)
			}
		}
	}
}

func (c *GameController) RunAutoFeederCalc() {
	hasLock := c.obtainLock()
	if hasLock {
		fetchRsp, err := c.MomentoClient.SortedSetFetchByScore(context.Background(), &momento.SortedSetFetchByScoreRequest{
			CacheName: serviceconfig.CacheName,
			SetName:   "score-board",
			Order:     momento.DESCENDING,
		})
		if err != nil {
			fmt.Printf("fatal error getting scoreboard for autofeeder run")
			return
		}

		switch r := fetchRsp.(type) {
		case *responses.SortedSetFetchHit:
			for _, user := range r.ValueStringElements() {
				fmt.Println("calculating auto feeder values for user + " + user.Value)
				autoFeedersRsp, err := c.MomentoClient.DictionaryFetch(context.Background(), &momento.DictionaryFetchRequest{
					CacheName:      serviceconfig.CacheName,
					DictionaryName: fmt.Sprintf("%s-auto-feeders", user.Value),
				})
				if err != nil {
					fmt.Printf("fatal error getting user %s autofeeder settings err=%+v\n", user.Value, err)
					continue
				}
				switch dr := autoFeedersRsp.(type) {
				case *responses.DictionaryFetchHit:
					for k, v := range dr.ValueMap() {
						autoFeederCount, err := strconv.Atoi(v)
						autoFeederType, err := strconv.Atoi(k)
						if err != nil {
							fmt.Printf("fatal error getting user %s autofeeder settings er=%+v\n", user.Value, err)
							continue
						}
						amountToIncrementScoreBy := float64(serviceconfig.AutoFeeders[autoFeederType].IncomePerMinute * autoFeederCount)
						fmt.Printf("incrementing score by %f for user %s\n", amountToIncrementScoreBy, user.Value)
						_, err = c.MomentoClient.SortedSetIncrementScore(context.Background(), &momento.SortedSetIncrementScoreRequest{
							CacheName: serviceconfig.CacheName,
							SetName:   "score-board",
							Value:     momento.String(user.Value),
							Amount:    float64(serviceconfig.AutoFeeders[autoFeederType].IncomePerMinute * autoFeederCount),
							Ttl: &utils.CollectionTtl{
								Ttl:        24 * time.Hour,
								RefreshTtl: true,
							},
						})
						if err != nil {
							fmt.Printf("fatal error incrementing user %s score err=%+v\n", user.Value, err)
							continue
						}
					}
				}
			}
		}
	} else {
		fmt.Println("did not obtain lease skipping auto feeder run")
	}
}

func (c *GameController) obtainLock() bool {
	rsp, err := c.MomentoClient.SetIfNotExists(context.Background(), &momento.SetIfNotExistsRequest{
		CacheName: serviceconfig.CacheName,
		Key:       momento.String("auto-feeder-lease"),
		Value:     momento.String(uuid.Must(uuid.NewRandom()).String()),
		Ttl:       5 * time.Second,
	})
	if err != nil {
		fmt.Println("fatal error occurred trying to get auto feeder lease err=" + err.Error())
		return false
	}

	switch rsp.(type) {
	case *responses.SetIfNotExistsStored:
		return true
	}

	return false
}
