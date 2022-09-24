package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"log"
	"net/http"
	"os"
	"sort"
)

type GameServerBuild struct {
	BuildId string
	Weight int
}

func main() {
	args := os.Args
	fileName := "thundernetes-buildalias.toml"
	helpMessage := "\n\tThundernetes build alias feature\n\n" +
	"1) create - generates a new build alias with the name provided." +
	" Each alias is made of one or more pairs of build id and weights (priority of assignment for allocation). Template for using:" +
	"\n\n\tbuildalias create <alias-name> <buildId1> <weight1> <buildId2> <weight2> ... <buildIdN> <weightN>\n\n" +
	"2) allocate - create a new game server instance for the build alias and session id provided. Template for using\n\n\t" +
	"buildalias allocate <alias-name> <sessionId>\n\n" +
	"3) help - Displays this message and exit"

	if len(args) == 1 {
		fmt.Println(helpMessage)
		return;
	}

	switch(args[1]) {
	case "allocate":
		AllocateForBuildAlias(args, fileName)

	case "help":
		fmt.Println(helpMessage)

	default:
		fmt.Println("Sorry, but the command "+ args[1] + " is not recognized.")
	}
}

func AllocateForBuildAlias(args []string, fileName string) {
	if len(args) < 6 {
		log.Fatal("Not enough arguments were provided.");
	}

	buildName := args[2]
	ipToAllocate := args[3]
	portToAllocate := args[4]
	sessionId := args[5]

	idx := 0
	aliasFound := false

	fmt.Println("Allocating a Game Server for " + buildName)
	ipToAllocate = "http://" + ipToAllocate + ":" + portToAllocate + "/api/v1/allocate"

	aliasMap := make(map[string][]map[string][]map[string]interface{})

	tomlFile, err := os.ReadFile(fileName)

	if err != nil {
		log.Fatal("Error while opening the file "+ fileName +". \n", err)
	}

	err = toml.Unmarshal(tomlFile, &aliasMap)

	if err != nil {
		log.Fatal("Error while unmarshaling the file "+ fileName +". \n", err)
	}

	if _, exists := aliasMap["alias"]; !exists {
		log.Fatal("FATAL. wrong format for TOML file: Missing alias array table name. ",
				  "Please refer to the template in thundernetes-buildalias.toml");
	}
	
	for i := 0; i < len(aliasMap["alias"]); i++ {
		if _, exists := aliasMap["alias"][i][buildName]; exists {
			idx = i
			aliasFound = true
			break;
		}
	}

	if !aliasFound {
		log.Fatal("Alias", buildName, "not declared on the toml config file")
	}
	
	aliasElements := aliasMap["alias"][idx][buildName]
	gameServers := make([]GameServerBuild, len(aliasElements))

	for i := 0; i < len(aliasElements); i++ {
		buildId := aliasElements[i]["buildId"]
		weight, ok := aliasElements[i]["weight"].(int64)

		if !ok {
			log.Fatal("Error while converting a weight for build id ",buildId)
		}

		gameServers[i].BuildId = fmt.Sprint(buildId)
		gameServers[i].Weight = int(weight)
	}

	// This is to override the default ascending behavior of sort library,
	// we declare a helper function to tell explicitely how to order our
	// slice (in this case in descending order by weight)
	sort.Slice(gameServers, func(i, j int) bool {
		return gameServers[i].Weight > gameServers[j].Weight
	});

	serverAllocated := false
	// Here, we iterate linearly since the elements were previously ordered and we're
	// giving priority to the builds with greater weights
	for i:= 0; i < len(gameServers); i++ {
		fmt.Println("Attempting to allocate...")

		reqBody := map[string]string{"buildID": gameServers[i].BuildId, "sessionID": sessionId}
		jsonData, err := json.Marshal(reqBody)

		if err != nil {
			log.Fatal("Error while marshaling the request. \n", err)
		}

		resp, err := http.Post(ipToAllocate, "application/json", bytes.NewBuffer(jsonData))

		if err != nil {
			log.Fatal("Error while allocating the game server. \n", err)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		// The post request returned a success as result code
		if resp.StatusCode == 200 {
			fmt.Println("Game server allocated successfully at IP Address: ", result["IPV4Address"], 
			            " with ports: ", result["Ports"])

			serverAllocated = true

			defer resp.Body.Close()
			break
		}

		fmt.Println("Allocation on server unsuccessful. Server Result code: ", resp.StatusCode)
		defer resp.Body.Close()
	}
	
	if !serverAllocated {
		fmt.Println("No server was available to allocate for build id", buildName)
	}
}