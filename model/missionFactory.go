package model

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/alexandrainst/agentlogic"
)

// // MissionType is a String
// type MissionType string

// // The types of mission abailable
// const (
// 	Find    MissionType = "find"
// 	Surveil             = "surveil"
// 	Measure             = "measure"
// 	Other               = "other"
// )

// type Mission struct {
// 	Description string
// 	MissionType MissionType
// 	AreaLink    string
// 	MetaNeeded  struct {
// 		MovementAxis   int
// 		SwarmSW        []string
// 		OnboardHW      []string
// 		DataCollection string
// 	}
// 	Goal struct {
// 		Do  string
// 		End string
// 	}
// 	GlobalArea *geojson.Feature
// 	AgentArea  string
// }

// type Waypoint struct {
// 	Wp      Vector
// 	Visited bool
// }

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
