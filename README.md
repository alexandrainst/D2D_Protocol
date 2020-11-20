# Drone-2-Drone Communication Protocol

This is a Proof-of-Concent for P2P communication between autonomous systems. It is based on LibP2Ps publish/subscribe framework and is created using Go modules.
Be awere, that being a PoC that there are quirks! 

## Prerequisites
This project has been created on MacOS 10.15.7, using Go 1.15. 
It is based on LibP2P and utilzes the following modules from Alexandra institute:
https://github.com/alexandrainst/agentlogic
https://github.com/alexandrainst/D2D-communication

These two modules handles, respectively the encapsulation of the data being worked on and the exact communication level. The structs in agentlogic can be extended to add extra fields as needed.

## Installing D2D Protocol
Make sure that you have Go >=1.15 installed: https://golang.org/doc/install
Run
 ```bash
	go test
```
to make sure that all modules are loaded.



## Using D2D Protocol

To use D2d Protocol, follow these steps:

Open one terminal window and a "controller", which is originally responsible to sending out a mission 
```
go run . -isController=true
```
This loads the metadata from 
```
metadata/ctrl.json
```
Then, in another terminal window start the agents for the swarm:
```
go run . -name=agent1
```
The 'name' variable refers to the randomX.json files in the metadata folder.
At the moment, there is Agent{1..3} possible, but feel free to add your own.

It is also possible to run the
```
startAgents.sh 
```
command with a name (similar to -name=agent1) and a number. This will start a number of random agents, based on the json file.

## Future work
1.	Integrate with hardware and/or SITLs to make it work with e.g. ArduPilot
2.	Improve on communication - minimize the number of messages sent
3.	Add device management
4.	Implement a goal function
5.	Implement the sending of data to a different endpoint in coherence with "goal"


## Contributing to D2D Protocol

To contribute to D2D Protocol, follow these steps:

1.  Fork this repository.
2.  Create a branch: `git checkout -b <branch_name>`.
3.  Make your changes and commit them: `git commit -m '<commit_message>'`
4.  Push to the original branch: `git push origin <project_name>/<location>`
5.  Create the pull request.


## Contributors

Thanks to the following people who have contributed to this project:

-   [@glagnar](https://github.com/glagnar) 
-   [@skovsen](https://github.com/skovsen) 