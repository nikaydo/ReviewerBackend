package queue

import (
	"log"
	"main/internal/ai"
	"main/internal/config"
	"main/internal/database"
	"main/internal/models"
	"strconv"
	"strings"
	"sync"
	"time"

	u "github.com/google/uuid"
)

func RunQueue(q *models.List, e config.Env, pg database.Database) {
	checkTimeout, _ := strconv.Atoi(e.EnvMap["QUEUE_TIMEOUT_CHECK"])
	sameTime, _ := strconv.Atoi(e.EnvMap["QUEUE_SAME_TIME_PROCESSED"])
	responseTimeout, _ := strconv.Atoi(e.EnvMap["QUEUE_TIMEOUT_BETWEN_RESPONSE"])
	wg := sync.WaitGroup{}
	for {
		q.Mutex.Lock()
		requests := make([]models.Enquiry, len(q.Request))
		copy(requests, q.Request)
		q.Mutex.Unlock()
		if len(requests) == 0 {
			time.Sleep(time.Duration(checkTimeout) * time.Millisecond)
			continue
		}
		time.Sleep(time.Duration(responseTimeout) * time.Millisecond)
		for i := range min(len(requests), sameTime) {
			req := requests[i]
			wg.Add(1)
			go func(req models.Enquiry) {
				defer wg.Done()
				answer, err := ai.Generate(
					req.Model,
					e.EnvMap["MISTRAL_API_KEY"],
					req.Request,
					req.System,
					req.Assistant,
					req.IsSystem,
					req.IsAssistant)
				if err != nil {
					log.Println(err)
					return
				}
				if err := selectType(req, pg, answer, e); err != nil {
					log.Println("generation: ", err)
				}
				if err := pg.InProgress("", req.Uuid); err != nil {
					log.Println("update in progress: ", err)
				}
				DropFromQueueByUUID(q, req.Uuid)
			}(req)
		}
		wg.Wait()
	}
}

func selectType(q models.Enquiry, pg database.Database, answer ai.ResponseFromAI, e config.Env) error {
	var erro error = nil
	switch q.Type {
	case 1:
		if err := pg.ReviewAdd(
			q.Uuid,
			q.Request,
			strings.ReplaceAll(answer.Response, "—", "-"),
			answer.Think,
			q.Model); err != nil {
			erro = err
		}
		if q.Memory {
			mem, err := pg.Recall(q.Uuid)
			if err != nil {
				log.Println("generation: ", err)
			}
			answer, err := ai.Generate(
				q.Model,
				e.EnvMap["MISTRAL_API_KEY"],
				q.Request,
				strings.Replace(pg.Env.EnvMap["PROMPT_FOR_MEMORIZATION"], "<memory>", mem, 1),
				"",
				true,
				false)
			if err != nil {
				erro = err
			}
			if err = pg.KeepInMind(q.Uuid, answer.Response); err != nil {
				erro = err
			}
		}
	case 2:
		if err := pg.ReviewTitleAdd(q.AskUuid, answer.Response, q.Request); err != nil {
			erro = err
		}
	}
	return erro
}

func WhereIAm(q *models.List, uuid u.UUID) models.MeInEnquiry {
	q.Mutex.Lock()
	defer q.Mutex.Unlock()
	All := len(q.Request)
	if All == 0 {
		return models.MeInEnquiry{Position: 0, Total: 0}
	}
	for i := range All {
		if q.Request[i].QueryUuid == uuid {
			return models.MeInEnquiry{Position: i + 1, Total: All}
		}
	}

	return models.MeInEnquiry{Position: 0, Total: 0}
}

func AddInQueue(q *models.List, en models.Enquiry) {
	q.Mutex.Lock()
	defer q.Mutex.Unlock()
	q.Request = append(q.Request, en)
}

func DropFromQueueByUUID(q *models.List, uuid string) {
	q.Mutex.Lock()
	defer q.Mutex.Unlock()
	for i, v := range q.Request {
		if v.Uuid == uuid {
			q.Request = append(q.Request[:i], q.Request[i+1:]...)
			return
		}
	}
}
