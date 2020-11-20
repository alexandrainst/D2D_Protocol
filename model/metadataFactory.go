package model

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/alexandrainst/agentlogic"
)

func loadMetadata(path string, isRand bool, randId int) agentlogic.Agent {
	log.Println("loading metadata from: " + path)
	agent := agentlogic.Agent{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("unable to read file: %v", err)
		panic(err)
	}
	json.Unmarshal([]byte(data), &agent)

	if isRand {
		agent.Nick = agent.Nick + "-" + strconv.Itoa(randId)
	}

	return agent
}

//GetMetadataForAgent returns all the meta needed
func GetMetadataForAgent(isSim *bool, isCtrl *bool, isRand bool, name *string) agentlogic.Agent {
	var path string

	var randId = 0
	if *isCtrl {
		path = "./metadata/ctrl.json"
	} else if !*isSim {
		//this is started on a physical device. We expect all infomration is found in file
		path = "./metadata/agent.json"
	} else {
		//this is started as a simulation. We randomly choose an metadata file
		rand.Seed(time.Now().UnixNano())
		var tmp string
		if len(*name) == 0 {
			tmp = "agent" + strconv.Itoa(rand.Intn(2)+1)
		} else {
			tmp = *name
		}
		randId = rand.Intn(1000)
		//pick between agent1 and 2

		//path = "./metadata/randomAgent" + strconv.Itoa(id) + ".json"
		path = "./metadata/random" + tmp + ".json"
	}
	return loadMetadata(path, isRand, randId)

}
