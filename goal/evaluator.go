package goal

import (
	"log"

	"alexandra.dk/D2D_Agent/workers"
	"github.com/alexandrainst/agentlogic"
	"github.com/paulmach/orb"
)

func EvaluteEndstate(goal agentlogic.Goal, position agentlogic.Vector) {
	log.Println("EVAL")
	log.Println(goal.Endgame)
	//implement different endgames here
	if goal.Endgame == "Gather" {
		goalPos := orb.Point{position.X, position.Y}
		goalLine := orb.MultiLineString{orb.LineString{goalPos}}
		workers.StateMux.Lock()
		//currPos := workers.MyState.Position

		m := agentlogic.Mission{
			Description: "Gather to goal",
			MissionType: workers.MyState.Mission.MissionType,
			Geometry:    goalLine,
		}
		workers.MyState.Mission = m
		workers.StateMux.Unlock()
		log.Println("Sending miss")
		workers.PrepareForMission(workers.MyState)
	}
}
