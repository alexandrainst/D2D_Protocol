package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"

	comm "github.com/alexandrainst/D2D-communication"
	"github.com/alexandrainst/agentlogic"
)

var controller *agentlogic.Agent

//var lastSeen time.Time

var HasCtrl = false

//var MyMission agentlogic.Mission
var SwarmMission *agentlogic.Mission

var buffersize = 128

var missionRunning = false
var ControllerDiscoveryChannel = make(chan *agentlogic.Agent, buffersize)

var MissionUpdateChannel = make(chan orb.LineString, buffersize)

var waypointMux = &sync.Mutex{}
var stateMux = &sync.Mutex{}

//NOTE: some of this work should maybe be removed as it is redundant
var mySelf *agentlogic.Agent
var myState *agentlogic.State

var height = float64(0)

var _isSim bool

func StartAgentWork(isSim *bool) {
	stateMux.Lock()
	myState = &agentlogic.State{
		ID:       mySelf.UUID,
		Mission:  *new(agentlogic.Mission),
		Battery:  mySelf.Battery,
		Position: mySelf.Position,
	}
	stateMux.Unlock()

	_isSim = *isSim
	//waiting for finding a controller before we start
	if !HasCtrl {
		//agent will not join the swarm unless is has a controller to begin with
		log.Println("Worker: Waiting to detect a controller")
		ctrl := <-ControllerDiscoveryChannel
		log.Println("Worker: Controller detected")
		SetController(ctrl)
	}

	sendAnnouncement()
	sendState()

	if mySelf.MovementDimensions > 2 {
		//if the agent can fly, it is set for specific height.
		//Not optimal, but usable for PoC
		height = 50
	}

	go func() {
		for {
			ctrl := <-ControllerDiscoveryChannel
			if !HasCtrl {
				log.Println("New controller found - not the one we started with")
				SetController(ctrl)
			}
		}
	}()

	if agentType == agentlogic.ContextAgent {
		go func() {

			log.Println("Waiting for mission")
			if *isSim {
				workAsSim()
			} else {
				//do another thing
			}

		}()
	}

}

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
		stateMux.Lock()
		select {
		case waypoints = <-MissionUpdateChannel:
			log.Printf("new number of waypoints: %d", len(waypoints))
			//log.Printf(MySelf.UUID+": New mission received with no of waypoints: %d \n", len(waypoints))
		default:
			//log.Println("nothing new")
		}

		currentIndex := myState.MissionIndex
		stateMux.Unlock()
		var nextIndex = 0
		if currentIndex < len(waypoints)-1 {
			nextIndex = currentIndex + 1
		}

		tmpWp := waypoints[nextIndex]
		nextWp := agentlogic.Vector{X: tmpWp.X(), Y: tmpWp.Y(), Z: height}
		stateMux.Lock()
		direction := nextWp.Sub(myState.Position)
		stateMux.Unlock()

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
			stateMux.Lock()
			myState.MissionIndex = nextIndex
			stateMux.Unlock()
			//and move on to next wp
			continue
		}

		//log.Println(myState.Position)
		//now we normalize
		normalizedDirection := direction.Normalize()
		//next we scale by delta
		newPos := normalizedDirection.MultiplyByScalar(deltaMovement)
		stateMux.Lock()
		myState.Position = myState.Position.Add(newPos)
		stateMux.Unlock()

		time.Sleep(250 * time.Millisecond)

	}
}

func sendAnnouncement() {
	go func() {
		for {

			comm.AnnounceSelf(mySelf)
			if *UseViz {
				go func(agent agentlogic.Agent) {
					m := comm.DiscoveryMessage{
						MessageMeta: comm.MessageMeta{MsgType: comm.DiscoveryMessageType, SenderId: mySelf.UUID, SenderType: agentType},
						Content:     agent,
					}
					comm.ChannelVisualization <- &m
				}(*mySelf)

			}
			time.Sleep(2 * time.Second)
		}
	}()

}

func PrepareForMission(state *agentlogic.State) {

	//log.Println(len(state.Mission.Geometry.(orb.MultiLineString)))
	waypointMux.Lock()
	missionRunning = false
	waypointMux.Unlock()
	stateMux.Lock()
	if state.Mission.Geometry == nil {
		log.Println(mySelf.UUID + ": No mission assigned")
		stateMux.Unlock()
		return
	} else {
		//log.Println(MySelf.UUID + ": New mission received")
	}
	ah := agentlogic.AgentHolder{Agent: *mySelf, State: *state}
	//agentPath, err := state.Mission.GeneratePath(*MySelf, 25)
	agentPath, err := state.Mission.GeneratePath(ah, 25)
	if err != nil {
		log.Println("Mission generation err:")
		log.Println(err)
	}
	//write to file
	//writeMissionToFile(*MySelf, state.Mission, agentPath)
	//end write
	if _isSim {
		//set the starting point as the first WP
		tmpWp := agentPath.(orb.MultiLineString)[0][0] //[0]
		myState.Position = agentlogic.Vector{X: tmpWp.X(), Y: tmpWp.Y(), Z: height}
		//log.Printf("setting startPoint %v, next wp: %v \n", myState.Position, agentPath.(orb.MultiLineString)[0][1])
	}
	//log.Printf("len agentPath: %d", len(agentPath.(orb.MultiLineString)[0]))
	state.Mission.Geometry = agentPath
	state.MissionIndex = 0

	select {
	case MissionUpdateChannel <- state.Mission.Geometry.(orb.MultiLineString)[0]:
		//log.Println("mission sent")
	default:
		//log.Println("no mission sent")
	}
	stateMux.Unlock()
	//log.Println("tween locks")
	waypointMux.Lock()
	missionRunning = true
	waypointMux.Unlock()
}

func sendState() {

	go func() {
		//now := int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
		for {
			// if agentType == agentlogic.ContextAgent {
			// 	tmp := int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
			// 	log.Printf(MySelf.UUID+": state time: %d\n", tmp-now)
			// 	now = tmp
			// }

			comm.SendState(myState)

			if *UseViz {
				go func(state agentlogic.State) {
					m := comm.StateMessage{
						MessageMeta: comm.MessageMeta{MsgType: comm.StateMessageType, SenderId: mySelf.UUID, SenderType: agentType},
						Content:     state,
					}

					comm.ChannelVisualization <- &m
				}(*myState)

			}

			time.Sleep(deltaForStateSend * time.Millisecond)
		}

	}()
}

func RemoveController() {
	log.Println("whoa...lost connection to controller!")
	controller = nil
	HasCtrl = false
}

func GetController() *agentlogic.Agent {
	return controller
}

func SetController(newController *agentlogic.Agent) {
	HasCtrl = true
	if controller == nil || controller.UUID != newController.UUID {
		log.Println("New controller discovered!")
		// log.Println("old: ")
		// log.Println(controller)
		// log.Println("new:")
		// log.Println(newController)
	}
	controller = newController
	//lastSeen = time.Now()
}

func writeMissionToFile(a agentlogic.Agent, m agentlogic.Mission, path orb.Geometry) {
	log.Println("writing mission to file")
	fc := geojson.NewFeatureCollection()
	fc.Append(geojson.NewFeature(m.Geometry))
	rawJSON, _ := fc.MarshalJSON()
	_ = ioutil.WriteFile(fmt.Sprintf("agentarea-%v.json", a.UUID), rawJSON, 0644)

	// path, err := m.GeneratePath(a, 25)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	fc = geojson.NewFeatureCollection()
	fc.Append(geojson.NewFeature(path))
	rawJSON, _ = fc.MarshalJSON()
	_ = ioutil.WriteFile(fmt.Sprintf("agentpath-%v.json", a.UUID), rawJSON, 0644)
}
