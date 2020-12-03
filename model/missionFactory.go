package model

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/alexandrainst/agentlogic"
)

func loadMission(path string) *agentlogic.Mission {
	log.Println("loading mission from: " + path)
	mission := new(agentlogic.Mission) //agentlogic.Mission{}
	//mission := agentlogic.Mission{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("unable to read file: %v", err)
		panic(err)
	}
	json.Unmarshal([]byte(data), mission)
	//mission.Geometry = agentlogic.LoadFeatures(mission.AreaLink).Geometry
	mission.LoadFeatures(mission.AreaLink)

	return mission
}

//GetMission returns mission
func GetMission() *agentlogic.Mission {

	path := "./metadata/mission.json"
	return loadMission(path)

}
