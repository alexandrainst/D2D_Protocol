package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"alexandra.dk/D2D_Agent/goal"
	"alexandra.dk/D2D_Agent/model"
	"alexandra.dk/D2D_Agent/workers"
	comm "github.com/alexandrainst/D2D-communication"
	"github.com/alexandrainst/agentlogic"
	"github.com/jinzhu/copier"
)

var UseViz *bool

var agents = make(map[string]agentlogic.AgentHolder)

var lostAgents = make(map[string][]string)
var agentsRecalculator = make(map[string]string)

var agentsMux = &sync.Mutex{}
var reorgMux = &sync.Mutex{}
var recalcMux = &sync.Mutex{}

var missionaireId *string

const zoomLevel = 25

const discoveryPath = "D2D_Discovery"
const statePath = "D2D_State"
const reorganizationPath = "D2D_Reorganization"
const recalculationPath = "D2D_Recalculation"

const timeForReorganizationWarning = 1      //secs
const timeForReorganizationWork = 2         //secs
const timeBetweenReorganisationCheck = 2000 //mili secs

func main() {

	// parse some flags to set our nickname and the room to join
	isCtrl := flag.Bool("isController", false, "Set this agent as controller")
	isSim := flag.Bool("isSimulation", true, "Set if this agent is started as a simulation")
	isViz := flag.Bool("useVisualization", true, "Set if this agent sends visualization data")
	logToFile := flag.Bool("logToFile", true, "Save log to file")
	name := flag.String("name", "", "filename for metadata. Is also nick for agent")
	isRand := flag.Bool("isRand", false, "append random id to nick")
	flag.Parse()

	UseViz = isViz
	if *isSim {
		log.Println("This agent is started in SIMULATION mode")
	} else {
		log.Println("This agent is started in REAL mode")
	}

	workers.MySelf = initAgent(isCtrl, isSim, isRand, name)

	if *logToFile {
		setupLogToFile(workers.MySelf.UUID)
	}

	log.Printf("Who Am I?\n %#v", workers.MySelf)
	//log.Printf("%#v", MySelf)

	startDiscoveryWork()
	workers.StartAgentWork(isSim, isViz)
	if *isCtrl {
		workers.MyState.Mission = *workers.SwarmMission
	}

	//add myself to agents map if not a controller
	if workers.AgentType != agentlogic.ControllerAgent {
		ah := &agentlogic.AgentHolder{
			Agent:     *workers.MySelf,
			State:     *workers.MyState,
			LastSeen:  time.Now().Unix(),
			AgentType: workers.AgentType,
		}
		agentsMux.Lock()

		agents[workers.MySelf.UUID] = *ah

		agentsMux.Unlock()
	}

	startStateWork()
	startReorganization()
	startMissionWork()
	startGoalWork()

	select {}
}

func initAgent(isCtrl *bool, isSim *bool, isRand *bool, name *string) *agentlogic.Agent {

	agent := model.GetMetadataForAgent(isSim, isCtrl, *isRand, name)

	if *isCtrl == true {
		workers.AgentType = agentlogic.ControllerAgent
		log.Println("This agent is started as a controller")
		missionaireId = &agent.UUID
		workers.SetController(&agent)
		workers.SwarmMission = model.GetMission()
		workers.SwarmMission.SwarmGeometry = workers.SwarmMission.Geometry
		log.Printf("bounds: %v", workers.SwarmMission.Geometry.Bound())
	} else {
		log.Println("This agent is started as a context unit")
		workers.AgentType = agentlogic.ContextAgent
		log.Printf(agent.Nick+" is starting at position: %v \n", agent.Position)
	}

	log.Println("Init communication with agentType: ", workers.AgentType)
	comm.InitD2DCommuncation(workers.AgentType)

	log.Println("Start registration on path: " + discoveryPath)
	//comm.InitRegistration(discoveryPath)
	comm.InitCommunicationType(discoveryPath, comm.DiscoveryMessageType)

	// log.Println("Start state on path: " + statePath)
	comm.InitCommunicationType(statePath, comm.StateMessageType)

	comm.InitCommunicationType(workers.GoalPath, comm.GoalMessageType)

	if *UseViz {
		log.Println("This agent sends visualization data")
		comm.InitVisualizationMessages(true)
	}
	//agent.UUID = comm.SelfId.Pretty()
	agent.UUID = agent.Nick
	return &agent
}

func startDiscoveryWork() {
	go func() {
		log.Println("Waiting to find companions")
		for {

			msg := <-comm.DiscoveryChannel
			agentId := msg.Content.UUID

			//log.Println(msg)
			agentsMux.Lock()
			_, ok := agents[agentId]
			agentsMux.Unlock()
			if ok {
				//agent already known
			} else {

				if msg.MessageMeta.SenderType == agentlogic.ControllerAgent {

					if workers.AgentType == agentlogic.ContextAgent && !workers.HasCtrl {
						//TODO: check that this is a controller, we trust
						log.Println("Found a controller - handing over mission privileges")
						workers.ControllerDiscoveryChannel <- &msg.Content
						go comm.InitCommunicationType(workers.MySelf.UUID, comm.MissionMessageType)
						missionaireId = &msg.Content.UUID
						if *UseViz {
							go func(agent agentlogic.Agent) {
								log.Println("ctrl id: " + agent.UUID)
								m := comm.DiscoveryMessage{
									MessageMeta: comm.MessageMeta{MsgType: comm.DiscoveryMessageType, SenderId: workers.MySelf.UUID, SenderType: workers.AgentType},
									Content:     agent,
								}
								comm.ChannelVisualization <- &m
							}(msg.Content)

						}
					} else if workers.AgentType == agentlogic.ControllerAgent {
						log.Println("Weird, another controller in swarm...we should panic!")
						log.Println(msg)
					}

				}

				log.Printf(workers.MySelf.UUID+": Found a buddy with nick: %s - adding to list \n", msg.Content.Nick)

				ah := &agentlogic.AgentHolder{
					Agent:     msg.Content,
					LastSeen:  time.Now().Unix(),
					AgentType: msg.MessageMeta.SenderType,
				}
				agentsMux.Lock()

				agents[agentId] = *ah

				agentsMux.Unlock()

				//setup mission channel to agent

				if missionaireId != nil && *missionaireId == workers.MySelf.UUID {
					//this is incredible slow and is called everytime a new peer is discovered
					//TODO: look into a faster approach
					sendMissions(agents)
				}

			}
		}

	}()
}

func startStateWork() {
	go func() {
		log.Println("Start looking for state transmission")
		for {
			msg := <-comm.StateChannel

			state := msg.Content

			agentId := state.ID
			agentsMux.Lock()
			if ah, ok := agents[agentId]; ok {

				if workers.MySelf.UUID == *missionaireId && ah.State.Mission.Geometry != nil {
					if state.Mission.Geometry == nil {
						//agent is sending a blank mission, while controller belives that it should have
						//resending...
						sendMissionToAgent(ah.Agent, ah.State.Mission)
					}
				}

				//agent already known
				ah.State = state
				ah.LastSeen = time.Now().Unix()

				agents[agentId] = ah

			}
			agentsMux.Unlock()
			//check to see if the agent was believed to be dead
			if _, ok := lostAgents[agentId]; ok {
				//we though it was dead - remove it from watchlist
				reorgMux.Lock()
				delete(lostAgents, agentId)
				reorgMux.Unlock()
			}
		}
	}()
}

func startReorganization() {
	log.Println("startReorganization")
	lostAgentsChanged := make(chan []string)

	comm.InitCommunicationType(reorganizationPath, comm.ReorganizationMessageType)

	//check local list of agents
	go func() {
		log.Println("Starting reorganization monitoring")
		//now := int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
		for {

			agentsMux.Lock()
			// if agentType == agentlogic.ContextAgent {
			// 	log.Println(MySelf.UUID + " is here")
			// }
			//if agentType == agentlogic.ControllerAgent {
			if *missionaireId == workers.MySelf.UUID {
				//tmp := int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
				//log.Printf("reorg time: %d\n", tmp-now)
				log.Printf(workers.MySelf.UUID+": Members of swarm: %d \n", len(agents))
				// if len(agents) < 10 {
				// 	log.Println("known Ids")
				// 	for id, _ := range agents {
				// 		log.Println(id)
				// 	}
				// }
				//now = tmp
			}

			for id, ah := range agents {
				if id == workers.MySelf.UUID {
					//log.Println("no need to check self")
					//no need to check if messages has come in from self
					continue
				}
				//log.Printf("ls: %v regor: %v combined: %v < now %v \n", ah.LastSeen, timeForReorganizationWarning, (ah.LastSeen + (timeForReorganizationWarning)), time.Now().Unix())
				if ah.LastSeen+(timeForReorganizationWarning) < time.Now().Unix() {
					if ah.LastSeen+(timeForReorganizationWork) < time.Now().Unix() {
						log.Printf(workers.MySelf.UUID+": has not seen agent with nick: %s for >%d seconds. Commencing reorganization \n", ah.Agent.Nick, timeForReorganizationWork)
						//close the connection for sending mission if controller
						if workers.AgentType == agentlogic.ControllerAgent {
							go comm.ClosePath(ah.Agent, comm.MissionMessageType)
						}
						delete(agents, id)
						reorgMux.Lock()
						lostAgents[id] = append(lostAgents[id], workers.MySelf.UUID)
						reorgMux.Unlock()
						//get ids of all living agents
						//has to be here because of deadlock otherwise
						livingIds := make([]string, 0, len(agents))
						for k := range agents {
							livingIds = append(livingIds, k)
						}
						lostAgentsChanged <- livingIds
						if ah.AgentType == agentlogic.ControllerAgent && workers.GetController() != nil && ah.Agent.UUID == workers.GetController().UUID {
							workers.RemoveController()
						}

						go comm.SendReorganization(ah.Agent, workers.MySelf.UUID)

						if *UseViz {
							go func(agent agentlogic.Agent) {
								m := comm.DiscoveryMessage{
									MessageMeta: comm.MessageMeta{MsgType: comm.ReorganizationMessageType, SenderId: workers.MySelf.UUID, SenderType: workers.AgentType},
									Content:     agent,
								}
								comm.ChannelVisualization <- &m
							}(ah.Agent)

						}
					} else {
						log.Printf("has not seen agent with nick: %s for >%d seconds \n", ah.Agent.Nick, timeForReorganizationWarning)
					}
				}
			}
			agentsMux.Unlock()

			time.Sleep(timeBetweenReorganisationCheck * time.Millisecond)
		}
	}()

	//receive notifications from other peers in network
	go func() {
		log.Println("Waiting to hear from peers about lost agents")
		for {
			msg := <-comm.ReorganizationChannel

			agentId := msg.Content.UUID
			log.Println(workers.MySelf.UUID + ": " + msg.MessageMeta.SenderId + ": somebody lost their way " + agentId + " !!")
			if agentId == workers.MySelf.UUID {
				log.Println(workers.MySelf.UUID + ": somebody else thinks that I'm gone. Stop The Count!")
				continue
			}

			sendToAgentsChanged := false
			reorgMux.Lock()
			if arr, ok := lostAgents[agentId]; ok {
				found := false
				for _, id := range arr {
					if id == msg.MessageMeta.SenderId {
						found = true
					}
				}
				if !found {
					arr = append(arr, msg.MessageMeta.SenderId)
					lostAgents[agentId] = arr
					sendToAgentsChanged = true
				}
			} else {
				lostAgents[agentId] = append(lostAgents[agentId], msg.MessageMeta.SenderId)
				sendToAgentsChanged = true
			}

			reorgMux.Unlock()
			if sendToAgentsChanged {
				agentsMux.Lock()
				livingIds := make([]string, 0, len(agents))
				for k := range agents {
					livingIds = append(livingIds, k)
				}
				agentsMux.Unlock()
				lostAgentsChanged <- livingIds
			}
		}
	}()

	// do reorganization if all living peers agree that an agent is gone
	go func() {
		for {
			livingIds := <-lostAgentsChanged

			//first we get all the ids of known, living, agents
			// agentsMux.Lock()
			// livingIds := make([]string, 0, len(agents))
			// for k := range agents {
			// 	livingIds = append(livingIds, k)
			// }
			// agentsMux.Unlock()
			//add own to list of living agents for comparison
			//livingIds = append(livingIds, mySelf.UUID)

			sort.Strings(livingIds)

			livingBytes, err := json.Marshal(livingIds)
			if err != nil {
				panic(err)
			}

			//second we see if all living peers agree on a missing agent
			// log.Println(MySelf.UUID)
			// log.Println(lostAgents)
			// log.Println(livingIds)
			reorgMux.Lock()
			updatedNeeded := false
			for id, arr := range lostAgents {
				sort.Strings(arr)
				notifiedBytes, err := json.Marshal(arr)
				if err != nil {
					panic(err)
				}

				res := bytes.Equal(livingBytes, notifiedBytes)

				if res {

					delete(lostAgents, id)
					//find the new agent to calculate new mission split
					log.Printf("Swarm agrees that %v is gone \n", id)
					updatedNeeded = true

					if _, ok := agentsRecalculator[id]; ok {
						//check to see if the dead agent were in the middle of a recalculation process
						//if so, remove it, as not to confuse remaining agents
						recalcMux.Lock()
						delete(agentsRecalculator, id)
						recalcMux.Unlock()
						log.Println(workers.MySelf.UUID + ": " + id + " was in a recalculation process. It is removed from that.")
					}

					//log.Printf("Agent with id %v will handle calculations for new missions \n", recalculatorId)
				} else {
					//log.Println("no agreement in swarm")
				}
			}
			if updatedNeeded {
				agentsMux.Lock()
				recalculatorId := findRecalculator(agents)
				agentsMux.Unlock()
				log.Printf("new recalculator id %v \n", recalculatorId)

				*missionaireId = recalculatorId
				var recalcAgent agentlogic.Agent
				if *missionaireId == workers.MySelf.UUID {
					recalcAgent = *workers.MySelf
				} else if workers.HasCtrl && recalculatorId == workers.GetController().UUID {
					recalcAgent = *workers.GetController()
				} else {
					recalcAgent = agents[recalculatorId].Agent
				}
				agentsRecalculator[workers.MySelf.UUID] = recalcAgent.UUID
				if len(agents) > 1 {

					comm.SendRecalculation(recalcAgent, workers.MySelf.UUID)
				} else {
					//only my self left
					m := comm.DiscoveryMessage{
						MessageMeta: comm.MessageMeta{MsgType: comm.RecalculatorMessageType, SenderId: workers.MySelf.UUID, SenderType: workers.AgentType},
						Content:     recalcAgent,
					}
					comm.RecalculationChannel <- &m
				}

				if *UseViz {
					go func(agent agentlogic.Agent) {
						m := comm.DiscoveryMessage{
							MessageMeta: comm.MessageMeta{MsgType: comm.RecalculatorMessageType, SenderId: workers.MySelf.UUID, SenderType: workers.AgentType},
							Content:     agent,
						}
						comm.ChannelVisualization <- &m
					}(recalcAgent)

				}
			}
			reorgMux.Unlock()

		}
	}()

	comm.InitCommunicationType(recalculationPath, comm.RecalculatorMessageType)
	go func() {
		log.Println("waiting to hear from peers about recalculation messages")
		for {

			msg := <-comm.RecalculationChannel
			log.Println("News about recalculation")

			agentId := msg.MessageMeta.SenderId

			agentsMux.Lock()
			noOfAgents := len(agents)
			agentsMux.Unlock()
			log.Printf("Realc message from: %s, about: %s", agentId, msg.Content.UUID)
			recalcMux.Lock()

			agentsRecalculator[agentId] = msg.Content.UUID

			if noOfAgents == len(agentsRecalculator) {
				swarmAgrees := true
				//if we have messages from all peers
				for _, recalcId := range agentsRecalculator {
					if recalcId != *missionaireId {
						log.Println("Peers do not agree on who is the new mission controller. Forcing a recalculate")
						log.Printf("recalID: %s vs missionId: %s", recalcId, *missionaireId)
						agentsMux.Lock()
						recalculatorId := findRecalculator(agents)
						recalcAgent := agents[recalculatorId]
						agentsMux.Unlock()
						log.Println("sending recalc: " + recalcAgent.Agent.UUID)
						comm.SendRecalculation(recalcAgent.Agent, workers.MySelf.UUID)
						swarmAgrees = false
						break
					}
				}
				if swarmAgrees {
					//peers are in agreement
					log.Println(workers.MySelf.UUID + ": Peers agree on who is responsible for mission planning: " + *missionaireId)
					//clear the list as they agree
					agentsRecalculator = make(map[string]string)
				}
				if *missionaireId == workers.MySelf.UUID {
					log.Println("whoaaa...I'm the new mission calculator. Better go to work")
					sendMissions(agents)

				}
			} else {
				missingVotes := noOfAgents - len(agentsRecalculator)
				log.Printf("still missing information from %d peers \n", missingVotes)
				agentsMux.Lock()
				for id, _ := range agents {
					_, ok := agentsRecalculator[id]
					if !ok {
						log.Printf("missing from: %s\n", id)
					}
				}
				agentsMux.Unlock()
			}
			recalcMux.Unlock()
		}
	}()
}

func startMissionWork() {
	log.Println("Waiting for mission")
	go func() {
		for {
			msg := <-comm.MissionChannel
			//log.Println("Got new mission")
			if missionaireId == nil {
				continue
			}
			//check to see if it comes from the controller we expects
			if msg.MessageMeta.SenderId == *missionaireId {
				//log.Println("got mission from expected controller")
			} else {
				//log.Println("got mission from UNKNOWN controller with Id: " + msg.MessageMeta.SenderId)
				continue
			}
			///update swarmMission is case changes has happened
			workers.SwarmMission = new(agentlogic.Mission)
			workers.SwarmMission.Description = msg.Content.Description
			workers.SwarmMission.MetaNeeded = msg.Content.MetaNeeded
			workers.SwarmMission.Goal = msg.Content.Goal
			workers.SwarmMission.Geometry = msg.Content.SwarmGeometry
			workers.StateMux.Lock()
			workers.MyState.Mission = msg.Content
			//we don't need it anymore
			workers.MyState.Mission.SwarmGeometry = nil
			workers.StateMux.Unlock()
			//log.Println("My mission has been updated")
			workers.PrepareForMission(workers.MyState)
		}
	}()
}

func startGoalWork() {
	log.Println("Waiting for goal")
	go func() {
		for {
			msg := <-comm.GoalChannel
			senderId := msg.MessageMeta.SenderId
			if _, ok := agents[senderId]; !ok {
				log.Println("Goal message received from unknown agent(" + senderId + "). It's being ignored!")
				continue
			}
			log.Println("Goal message received from: " + senderId + "!")
			//TODO: compare the recived goal to own goal, to make sure that it fits

			goal.EvaluteEndstate(msg.Content, msg.Position)
		}
	}()
}

func recalculateMission(agents map[string]agentlogic.AgentHolder) map[string]agentlogic.AgentHolder {
	//TODO: Handle if ctrl disappears and context agent takes over - right now, a context will not be part of the new swarm mission
	if workers.SwarmMission == nil {
		log.Println("Not able to calculate missions, as swarm mission is nil!!")
		return nil
	}
	agentsMux.Lock()

	missions, err := agentlogic.ReplanMission(*workers.SwarmMission, agents, zoomLevel)

	agentsMux.Unlock()
	if err != nil {
		log.Println("Not able to plan missions - panicking")
		panic(err)
	}

	agentsMux.Lock()
	for id, ah := range agents {
		agentMission := missions[id]
		agentMission.SwarmGeometry = workers.SwarmMission.Geometry
		agentMission.Description = agentMission.Description + " Sent from " + workers.MySelf.Nick
		ah.State.Mission = agentMission
		agents[id] = ah
	}
	agentsMux.Unlock()

	return agents
}

func sendMissions(agents map[string]agentlogic.AgentHolder) {
	log.Println("Starting sending new missions")

	if workers.AgentType == agentlogic.ControllerAgent && len(agents) == 0 {
		log.Println("No agents to send mission to! I'm all alone")
		return
	}

	tmp := recalculateMission(agents)

	var tmpAgents = make(map[string]agentlogic.AgentHolder)

	agentsMux.Lock()
	agents = tmp
	for id, ah := range agents {
		tah := agentlogic.AgentHolder{}
		//log.Printf("miss %v\n", len(ah.State.Mission.Geometry.(orb.Polygon)[0]))

		copier.Copy(&tah, ah)

		tmpAgents[id] = tah
		if id != workers.MySelf.UUID {
			comm.InitCommunicationType(id, comm.MissionMessageType)
		}
	}
	agentsMux.Unlock()

	broadcastMission(tmpAgents)
}

func broadcastMission(tmpAgents map[string]agentlogic.AgentHolder) {
	log.Println("broadcasting")
	//time.Sleep(2 * time.Second)
	agentsMux.Lock()
	var tmpIds []string
	for id, ah := range tmpAgents {
		if ah.State.Mission.Geometry == nil {
			log.Printf("mission is nil - not sending %s \n", id)
			//continue
		}
		tmpIds = append(tmpIds, id)
		sendMissionToAgent(ah.Agent, ah.State.Mission)
	}
	agentsMux.Unlock()
	log.Printf(workers.MySelf.UUID+": Sent mision to %v \n", tmpIds)

}

func sendMissionToAgent(agent agentlogic.Agent, mission agentlogic.Mission) {

	//var channelPath string
	channelPath := *missionaireId
	if *missionaireId == workers.MySelf.UUID {
		channelPath = agent.UUID
	}
	//log.Println("Sending mission to " + channelPath)

	go comm.SendMission(workers.MySelf.UUID, &mission, channelPath)

	if *UseViz {
		go func(vizMission agentlogic.Mission) {
			m := comm.MissionMessage{
				MessageMeta: comm.MessageMeta{MsgType: comm.MissionMessageType, SenderId: agent.UUID, SenderType: workers.AgentType},
				Content:     vizMission,
			}
			comm.ChannelVisualization <- &m
		}(mission)
	}
}

//atm we just choose the agent in the swarm with lowest id - it's trivial, i know
func findRecalculator(agents map[string]agentlogic.AgentHolder) string {
	if workers.HasCtrl {
		log.Println("Controller still in swarm - no need to change who handles missions")
		return workers.GetController().UUID
	}
	if len(agents) == 0 {
		//only one left
		return workers.MySelf.UUID
	}

	//agentsMux.Lock()
	keys := make([]string, 0, len(agents))
	for k := range agents {
		keys = append(keys, k)

	}

	//agentsMux.Unlock()

	sort.Strings(keys)

	return keys[0]
}

func setupLogToFile(id string) {

	path := "logs/agent-" + id + ".log"
	// alreadyExists := false
	// if fileExists(path) {
	// 	alreadyExists = true
	// 	os.Remove(path)

	// }

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	mw := io.MultiWriter(os.Stdout, file)

	log.SetOutput(mw)
	log.Println("------------------------------------------------------------------------------------------------")
	log.Println("Start logging to file")
	// if alreadyExists {
	// 	log.Println("Log file already existed. Have purged")
	// }
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

/*
TODO:

	visualize messages
		recalc from agents/controller
		reorg from  agents/controller
		maybe discovery of missing peer?

	test:
		controller comes back
		what happens to the drone that takes over the controller role?
	remove and add same agent (after timeout) to check what happens
	use authentication

	Goal function


*/
