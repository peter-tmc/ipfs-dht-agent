package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"modified-agent-ipfs/config"
	agent "modified-agent-ipfs/worker_agent"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Main struct {
}

func main() {
	ctx := context.Background()
	rand.Seed(time.Now().UnixNano())
	var zapCfg zap.Config

	rawJSON := []byte(`{
		"encoding": "json",
		"level": "info",
		"outputPaths": ["./logs/log.txt"],
		"errorOutputPaths": ["./logs/log.txt"],
		"encoderConfig": {
		  "messageKey": "main",
		  "levelKey": "level",
		  "levelEncoder": "lowercase"
		}
	  }`)
	if err := json.Unmarshal(rawJSON, &zapCfg); err != nil {
		panic(err)
	}
	logger, err := zapCfg.Build()
	if err != nil {
		panic(err)
	}
	logger.Info("Starting at " + time.Now().UTC().String())

	ctx = context.WithValue(ctx, "zapLogger", logger)

	keys, err := readKeys(logger)
	if err != nil {
		panic(err)
	}

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetConfigType("yml")
	var cfg config.Configuration
	err = viper.ReadInConfig()
	if err != nil {
		logger.Panic(err.Error())
	}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		logger.Panic(err.Error())
	}
	logger.Info("viper set up done")

	w := agent.NewAgent(ctx, cfg)
	logger.Info("Providing values")
	go func() {
		for {
			i := rand.Intn(255)
			id := uuid.New()
			//logger.Info("Finding providers for key with index " + strconv.Itoa(i) + " key is " + string(keys[i]))
			providers, err := w.FindProviders(ctx, keys[i], i, id)
			if err != nil {
				logger.Error(err.Error())
			}
			str := "Length of values " + strconv.Itoa(len(providers)) + "; values are"
			for _, v := range providers {
				str = str + " " + v.String()
			}
			logger.Info(str)
		}
	}()

	provideAllValues(ctx, keys, w, logger)
	count := 0
	for {
		time.Sleep(5 * time.Minute)
		count++
		if count%288 == 0 { //if 288*5 minutes have passed which is 24 hours reprovide values
			provideAllValues(ctx, keys, w, logger)
		}
	}
}

//TODO : passar um logger diferente para este para ir para outro ficheiro
func provideAllValues(ctx context.Context, keys [256][]byte, a *agent.Agent, logger *zap.Logger) {
	keyIndexes := a.GetIndexes()
	logger.Info("Started providing " + strconv.Itoa(len(keyIndexes)) + " keys")
	str := "Going to provide key indexes:"
	for _, v := range keyIndexes {
		str = str + " " + strconv.Itoa(v)
	}
	logger.Info(str)
	t1 := time.Now()
	count := 0
	finish := make(chan int)
	for _, v := range keyIndexes {
		id, err := uuid.NewUUID()
		if err != nil {
			logger.Error(err.Error())
		}
		logger.Info("Providing key with index " + strconv.Itoa(v) + " key is " + string(keys[v]) + " ID is " + id.String())
		go func(v int, finish chan int) {
			err := a.ProvideVal(ctx, keys[v], v, id)
			if err != nil {
				panic(err)
			}
			finish <- v
		}(v, finish)
	}
	for {
		<-finish
		count++
		if count == len(keyIndexes) {
			break
		}
	}
	t2 := time.Now()
	logger.Info("Successful in providing keys, took " + (t2.Sub(t1)).String())
}

func readKeys(logger *zap.Logger) ([256][]byte, error) {
	open, err := os.OpenFile("keys.txt", os.O_RDONLY, 0755)
	if err != nil {
		panic(err)
	}
	var res [256][]byte
	aux := make([]byte, 32)
	at, err := open.ReadAt(aux, int64(0))
	if err != nil {
		panic(err)
	}
	if at != 32 {
		panic("Leu menos que 32")
	}
	res[0] = aux

	for i := 1; i <= 255; i++ {
		aux2 := make([]byte, 32)
		readAt, err := open.ReadAt(aux2, int64(i*34))
		logger.Debug("i is " + strconv.Itoa(i) + " offset is " + strconv.FormatInt(int64(i*32+2), 10))
		if err != nil {
			panic(err)
		}
		if readAt != 32 {
			panic("Leu menos que 32")
		}
		res[i] = aux2
	}
	err = open.Close()
	if err != nil {
		panic(err)
	}
	return res, nil
}
