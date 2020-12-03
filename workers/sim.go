package workers

import (
	"log"
	"time"

	comm "github.com/alexandrainst/D2D-communication"

	"github.com/alexandrainst/agentlogic"
)

func workAsSim() {
	log.Println("Starting to work as sim")
	//the length of step an agent can move in a sim
	var deltaMovement = float64(0.000005)
	//var wayPoints = *line
	waypoints := <-MissionUpdateChannel
	log.Println("Mission received")
	log.Printf("number of waypoints: %d", len(waypoints))

	for {

		waypointMux.Lock()
		if !missionRunning {
			waypointMux.Unlock()
			time.Sleep(750 * time.Millisecond)
			continue
			//could probably be done more effectively
		}

		//wayPoints = *line

		waypointMux.Unlock()
		StateMux.Lock()
		select {
		case waypoints = <-MissionUpdateChannel:
			log.Printf("new number of waypoints: %d", len(waypoints))
			//log.Printf(MySelf.UUID+": New mission received with no of waypoints: %d \n", len(waypoints))
		default:
			//log.Println("nothing new")
		}

		currentIndex := MyState.MissionIndex
		StateMux.Unlock()
		var nextIndex = 0
		if currentIndex < len(waypoints)-1 {
			nextIndex = currentIndex + 1
		}

		tmpWp := waypoints[nextIndex]
		nextWp := agentlogic.Vector{X: tmpWp.X(), Y: tmpWp.Y(), Z: height}
		StateMux.Lock()
		direction := nextWp.Sub(MyState.Position)
		StateMux.Unlock()
		// if MySelf.UUID == "Agent1" {
		// 	log.Println("debug send")
		// 	vec := agentlogic.Vector{X: 0, Y: 0, Z: 0}
		// 	comm.SendGoalFound(MySelf.UUID, MyState.Mission.Goal, vec, GoalPath)
		// }
		if _useViz {
			select {
			case vm := <-comm.GetVizChannel().Messages:
				if len(vm.GoalMessage.MessageMeta.SenderId) > 0 && vm.GoalMessage.MessageMeta.SenderId == MySelf.UUID {
					found := CheckGoal(MyState.Mission.Goal, MyState.Position, vm.GoalMessage.Poi)
					if found {
						//found the goal!
						log.Println("I FOUND the goal. I win!")
						waypointMux.Lock()
						missionRunning = false
						waypointMux.Unlock()
						comm.SendGoalFound(MySelf.UUID, MyState.Mission.Goal, vm.GoalMessage.Position, GoalPath)

					}
				}
			default:
				//log.Println("no gola")
			}

		}

		//log.Println(direction.Length())
		//log.Printf("dirLen: %f, deltaMov: %f \n currentPos: %v nextWP: %v", direction.Length(), deltaMovement, myState.Position, nextWp)
		if direction.Length() < deltaMovement {
			//log.Println("Getting next WP")
			if nextIndex == 0 {
				//log.Println("at starting point. Waiting two seconds to start mission")
				time.Sleep(2 * time.Second)
				//log.Println("STARTING MISSION")
				//log.Println(nextWp)
			} else {
				//log.Printf(MySelf.UUID+": at wp %d, moving to next: %v \n", nextIndex, nextWp)
			}
			//if we are within one step of our goal, we mark it as completed
			StateMux.Lock()
			MyState.MissionIndex = nextIndex
			StateMux.Unlock()
			//and move on to next wp
			continue
		}

		//log.Println(myState.Position)
		//now we normalize
		normalizedDirection := direction.Normalize()
		//next we scale by delta
		newPos := normalizedDirection.MultiplyByScalar(deltaMovement)
		StateMux.Lock()
		MyState.Position = MyState.Position.Add(newPos)
		StateMux.Unlock()

		time.Sleep(175 * time.Millisecond)

	}
}
