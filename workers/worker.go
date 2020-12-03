package workers

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

var AgentType agentlogic.AgentType

var controller *agentlogic.Agent

const GoalPath = "D2D_Goal"

var HasCtrl = false

//var MyMission agentlogic.Mission
var SwarmMission *agentlogic.Mission

var buffersize = 128

var missionRunning = false
var ControllerDiscoveryChannel = make(chan *agentlogic.Agent, buffersize)

var MissionUpdateChannel = make(chan orb.LineString, buffersize)

var waypointMux = &sync.Mutex{}
var StateMux = &sync.Mutex{}

//NOTE: some of this work should maybe be removed as it is redundant
var MySelf *agentlogic.Agent
var MyState *agentlogic.State

const deltaForStateSend = 750 //mili secs

var _isSim bool
var _useViz bool

//dummy
var height = float64(0)

func StartAgentWork(isSim *bool, useViz *bool) {
	StateMux.Lock()
	MyState = &agentlogic.State{
		ID:       MySelf.UUID,
		Mission:  *new(agentlogic.Mission),
		Battery:  MySelf.Battery,
		Position: MySelf.Position,
	}
	StateMux.Unlock()

	_isSim = *isSim
	_useViz = *useViz
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

	if MySelf.MovementDimensions > 2 {
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

	if AgentType == agentlogic.ContextAgent {
		go func() {

			log.Println("Waiting for mission")
			if *isSim {
				workAsSim()
			} else {
				//do another thing
				workAsPhysical()
			}

		}()
	}

}

func sendAnnouncement() {
	go func() {
		for {

			comm.AnnounceSelf(MySelf)
			if _useViz {
				go func(agent agentlogic.Agent) {
					m := comm.DiscoveryMessage{
						MessageMeta: comm.MessageMeta{MsgType: comm.DiscoveryMessageType, SenderId: MySelf.UUID, SenderType: AgentType},
						Content:     agent,
					}
					comm.ChannelVisualization <- &m
				}(*MySelf)

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
	StateMux.Lock()
	if state.Mission.Geometry == nil {
		log.Println(MySelf.UUID + ": No mission assigned")
		StateMux.Unlock()
		return
	} else {
		//log.Println(MySelf.UUID + ": New mission received")
	}
	ah := agentlogic.AgentHolder{Agent: *MySelf, State: *state}
	//agentPath, err := state.Mission.GeneratePath(*MySelf, 25)
	//check if only a point - if so, it's a goal
	agentPath := state.Mission.Geometry
	_, ok := state.Mission.Geometry.(orb.Polygon)
	if ok == true {

		var err error
		agentPath, err = state.Mission.GeneratePath(ah, 25)
		if err != nil {
			log.Println("Mission generation err:")
			log.Println(err)
		}
		if _isSim {
			//set the starting point as the first WP
			tmpWp := agentPath.(orb.MultiLineString)[0][0] //[0]
			MyState.Position = agentlogic.Vector{X: tmpWp.X(), Y: tmpWp.Y(), Z: height}
			//log.Printf("setting startPoint %v, next wp: %v \n", myState.Position, agentPath.(orb.MultiLineString)[0][1])
		}
	}

	//write to file
	//writeMissionToFile(*MySelf, state.Mission, agentPath)
	//end write

	//log.Printf("len agentPath: %d", len(agentPath.(orb.MultiLineString)[0]))
	state.Mission.Geometry = agentPath
	state.MissionIndex = 0

	log.Println(MyState.Position)

	select {
	case MissionUpdateChannel <- state.Mission.Geometry.(orb.MultiLineString)[0]:
		//log.Println("mission sent")
	default:
		//log.Println("no mission sent")
	}
	StateMux.Unlock()
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

			comm.SendState(MyState)

			if _useViz {
				go func(state agentlogic.State) {
					m := comm.StateMessage{
						MessageMeta: comm.MessageMeta{MsgType: comm.StateMessageType, SenderId: MySelf.UUID, SenderType: AgentType},
						Content:     state,
					}

					comm.ChannelVisualization <- &m
				}(*MyState)

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
