package main

import (
	"context"
	"crynux_relay/api"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/migrate"
	"crynux_relay/service"
	"crynux_relay/tasks"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	if err := config.InitConfig(""); err != nil {
		print("Error reading config file")
		print(err.Error())
		os.Exit(1)
	}

	conf := config.GetConfig()

	if err := config.InitLog(conf); err != nil {
		print("Error initializing log")
		print(err.Error())
		os.Exit(1)
	}

	if err := config.InitDB(conf); err != nil {
		log.Errorln(err.Error())
		os.Exit(1)
	}

	startDBMigration()

	if err := blockchain.Init(context.Background()); err != nil {
		log.Fatalln(err)
	}
	if err := service.InitTaskFeeCache(context.Background(), config.GetDB()); err != nil {
		log.Fatalln(err)
	}
	if err := service.InitTaskQuotaCache(context.Background(), config.GetDB()); err != nil {
		log.Fatalln(err)
	}
	if err := service.InitDelegatorShareCache(context.Background(), config.GetDB()); err != nil {
		log.Fatalln(err)
	}
	if err := service.InitDelegationCaches(context.Background(), config.GetDB()); err != nil {
		log.Fatalln(err)
	}
	if err := service.InitSelectingProb(context.Background(), config.GetDB()); err != nil {
		log.Fatalln(err)
	}

	tm := blockchain.NewTransactionManager(config.GetDB())
	tm.Start(context.Background())

	service.StartBlockListener(context.Background())
	go service.StartTaskProcesser(context.Background())
	go service.StartTaskQuotaSync(context.Background(), config.GetDB())
	go service.StartTaskFeeSync(context.Background(), config.GetDB())
	// go tasks.ProcessTasks(context.Background())
	go tasks.StartSyncNetwork(context.Background())
	go tasks.StartStatsTaskCount(context.Background())
	go tasks.StartStatsTaskExecutionTimeCount(context.Background())
	go tasks.StartStatsTaskUploadResultTimeCount(context.Background())
	go tasks.StartStatsTaskWaitingTimeCount(context.Background())
	go tasks.StartStatsNodeScores(context.Background())
	go tasks.StartStatsNodeStakings(context.Background())
	go tasks.StartStatsNodeDelegatorCount(context.Background())

	startServer()
}

func startServer() {
	conf := config.GetConfig()

	app := api.GetHttpApplication(conf)
	address := fmt.Sprintf("%s:%s", conf.Http.Host, conf.Http.Port)

	log.Infoln("Starting application server...")

	if err := app.Run(address); err != nil {
		log.Errorln(err.Error())
		os.Exit(1)
	}
}

func startDBMigration() {

	migrate.InitMigration(config.GetDB())

	if err := migrate.Migrate(); err != nil {
		log.Errorln(err.Error())
		if err = migrate.Rollback(); err != nil {
			log.Errorln(err.Error())
		}
		os.Exit(1)
	}

	log.Infoln("DB migrations are done!")
}
