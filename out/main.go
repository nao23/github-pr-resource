package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type P struct {
	Status  string `json:"status"`
	Context string `json:"context"`
	Path    string `json:"path"`
}

type Input struct {
	Source struct {
		Owner       string `json:"owner"`
		Repo        string `json:"repo"`
		AccessToken string `json:"access_token"`
		Org         string `json:"org"`
		Label       string `json:"label"`
	} `json:"source"`
	Params P `json:"params"`
}

type Ver struct {
	Ref    string `json:"ref"`
	Number string `json:"number"`
}

type Output struct {
	Version Ver `json:"version"`
}

func main() {
	//takes input from stdin in JSON
	decoder := json.NewDecoder(os.Stdin)

	var inp Input
	err := decoder.Decode(&inp)

	if err != nil {
		log.Fatal(err)
	}

	//create client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: inp.Source.AccessToken})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	//get ref header from directory
	cmd := exec.Command("/find_hash.sh", os.Args[1], inp.Params.Path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	b, err := cmd.Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + " : " + stderr.String())
	}

	ref := string(b[:len(b)-1])

	ATC_EXTERNAL_URL := os.Getenv("ATC_EXTERNAL_URL")
	BUILD_TEAM_NAME := os.Getenv("BUILD_TEAM_NAME")
	BUILD_PIPELINE_NAME := os.Getenv("BUILD_PIPELINE_NAME")
	BUILD_JOB_NAME := os.Getenv("BUILD_JOB_NAME")
	BUILD_NAME := os.Getenv("BUILD_NAME")

	targetURL := ATC_EXTERNAL_URL + "/teams/" + BUILD_TEAM_NAME + "/pipelines/" + BUILD_PIPELINE_NAME + "/jobs/" + BUILD_JOB_NAME + "/builds/" + BUILD_NAME
	description := "concourse-ci build : " + inp.Params.Status
	gitContext := "concourse-ci " + BUILD_JOB_NAME + " " + BUILD_NAME

	//update status of the pr
	newStatus := &github.RepoStatus{
		State:       github.String(inp.Params.Status),
		TargetURL:   &targetURL,
		Description: &description,
		Context:     &gitContext,
	}

	_, _, err = client.Repositories.CreateStatus(context.Background(), inp.Source.Owner, inp.Source.Repo, ref, newStatus)
	if err != nil {
		log.Fatal(err)
	}

	//find pr_no. this need to be printed to stdout
	cmd = exec.Command("/fetch_pr.sh", os.Args[1], inp.Params.Path)
	cmd.Stderr = &stderr

	b, err = cmd.Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + " : " + stderr.String())
	}

	if len(b) == 0 {
		log.Fatal("Commit Squashed")
	}

	num := string(b[:len(b)-1])

	id, err := strconv.Atoi(num)

	if err != nil {
		log.Fatal(err)
	}

	//_, _, err = client.Issues.ListLabelsByIssue(context.Background(), inp.Source.Owner, inp.Source.Repo, id, nil)

	//if err != nil {
	//	log.Println("err", err.Error())
	//}

	//get pr from api and remove label
	_, err = client.Issues.RemoveLabelForIssue(context.Background(), inp.Source.Owner, inp.Source.Repo, id, inp.Source.Label)

	if err != nil {
		//TODO: thie code removes label from PR, but also shows error 404. IDK why
		log.Println(err.Error())
	}

	//prepare output format
	op := Output{
		Version: Ver{
			Ref:    ref,
			Number: num,
		},
	}

	b, err = json.Marshal(op)

	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stdout.Write(b)
	if err != nil {
		log.Fatal(err)
	}
}
