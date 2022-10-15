//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/.github.
//
// SPDX-License-Identifier: Apache-2.0
//
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v47/github"
	"golang.org/x/oauth2"
	"io/ioutil"
	"os"
	"strings"
)

// LabelSpec describes a label with its basic properties.
type LabelSpec struct {
	Name        string `json:"name"`
	Replaces    string `json:"replaces,omitempty"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// Labels contains label specs and processing directives.
type Labels struct {
	DeleteObsolete bool        `json:"delete-obsolete"`
	Desired        []LabelSpec `json:"desired"`
}

// Config contains the configuration used to control the execution of the command.
type Config struct {
	Repositories []string `json:"repositories"`
	Labels       Labels   `json:"labels"`
}

// readConfig parses the JSON configuration file expected to be located in the working directory.
func readConfig() (*Config, error) {
	file, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = json.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// getLabelsInRepo fetches the list of labels defined on a repository.
func getLabelsInRepo(ctx context.Context, client *github.Client, org string, repo *github.Repository) (map[string]github.Label, error) {
	labels, _, err := client.Issues.ListLabels(ctx, org, repo.GetName(), nil)
	if err != nil {
		return nil, err
	}
	labelByName := make(map[string]github.Label)
	for _, label := range labels {
		labelByName[label.GetName()] = *label
	}
	return labelByName, nil
}

// labelNeedsUpdate checks whether the actual label needs to be updated to be equal to the desired label.
func labelNeedsUpdate(desired LabelSpec, actual github.Label) bool {
	name := desired.Name != actual.GetName()
	if name {
		fmt.Printf("Label name deviation (old: %s, new: %s)\n", *actual.Name, desired.Name)
	}
	desc := desired.Description != actual.GetDescription()
	if desc {
		fmt.Printf("Label description deviation (old: %s, new: %s)\n", *actual.Description, desired.Description)
	}
	color := desired.Color != actual.GetColor()
	if color {
		fmt.Printf("Label color deviation (old: %s, new: %s)\n", *actual.Color, desired.Color)
	}
	return name || desc || color
}

// updateLabelsInRepo performs the steps required to make the set of labels in a repository comply with the desired set
// of labels as defined in the configuration file. Labels get either replaced ("replaces" key), updated, or newly
// created. In case "delete-obsolete" is true, any label that is not among the set of desired labels will be deleted.
func updateLabelsInRepo(ctx context.Context, client *github.Client, org string, repo *github.Repository, labels Labels) error {
	fmt.Printf("\nUpdating repository: %s\n", *repo.Name)
	actual, err := getLabelsInRepo(ctx, client, org, repo)
	if err != nil {
		return err
	}
	for _, label := range labels.Desired {
		if actualLabel, ok := actual[label.Name]; ok {
			if labelNeedsUpdate(label, actualLabel) {
				err := updateLabel(ctx, client, org, repo, label, actualLabel)
				if err != nil {
					return err
				}
			}
		} else {
			if replacedLabel, ok := actual[label.Replaces]; ok {
				err := updateLabel(ctx, client, org, repo, label, replacedLabel)
				if err != nil {
					return err
				}
				delete(actual, replacedLabel.GetName())
			} else {
				err := createLabel(ctx, client, org, repo, label)
				if err != nil {
					return err
				}
			}
		}
	}
	if labels.DeleteObsolete {
		keep := make(map[string]bool)
		for _, label := range labels.Desired {
			keep[label.Name] = true
		}
		for _, label := range actual {
			name := label.GetName()
			if !keep[name] {
				err := deleteLabel(ctx, client, org, repo, err, name)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// deleteLabel deletes the given label if confirmed by the user.
func deleteLabel(ctx context.Context, client *github.Client, org string, repo *github.Repository, err error, name string) error {
	confirmed, err := askForConfirmation(fmt.Sprintf("Delete obsolete label: %s?", name))
	if err != nil {
		return err
	}
	if confirmed {
		_, err = client.Issues.DeleteLabel(ctx, org, *repo.Name, name)
		if err != nil {
			return err
		}
	}
	return nil
}

// createLabel creates a new label according to the given label specification.
func createLabel(ctx context.Context, client *github.Client, org string, repo *github.Repository, label LabelSpec) error {
	fmt.Printf("Creating missing label: %s\n", label.Name)
	l := github.Label{
		Name:        &label.Name,
		Color:       &label.Color,
		Description: &label.Description,
	}
	_, _, err := client.Issues.CreateLabel(ctx, org, *repo.Name, &l)
	return err
}

// updateLabel modifies / updates a given label to match the given label specification.
func updateLabel(ctx context.Context, client *github.Client, org string, repo *github.Repository, desiredLabel LabelSpec, actualLabel github.Label) error {
	fmt.Printf("Update existing label: %s\n", actualLabel.GetName())
	l := github.Label{
		ID:          actualLabel.ID,
		URL:         actualLabel.URL,
		Name:        &desiredLabel.Name,
		Color:       &desiredLabel.Color,
		Description: &desiredLabel.Description,
		Default:     actualLabel.Default,
		NodeID:      actualLabel.NodeID,
	}
	_, _, err := client.Issues.EditLabel(ctx, org, *repo.Name, actualLabel.GetName(), &l)
	return err
}

// askForConfirmation prompts the user to confirm or abort an action.
func askForConfirmation(question string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", question)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "y" {
			return true, nil
		} else if answer == "n" {
			return false, nil
		}
	}
}

// OrgName is the GitHub organisation name to operate on.
const OrgName = "carbynestack"

func main() {
	ctx := context.Background()

	// Create the GitHub client using the given access token
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("No GitHub access token provided using GITHUB_TOKEN environment variable")
		os.Exit(1)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Read in the configuration and prepare auxiliary data structures
	config, err := readConfig()
	if err != nil {
		fmt.Printf("Error reading configuration: %v", err)
		os.Exit(2)
	}
	includedRepos := map[string]bool{}
	for _, repository := range config.Repositories {
		includedRepos[repository] = true
	}

	// Traverse repos in org and updates labels if repo is included
	repos, _, err := client.Repositories.ListByOrg(ctx, OrgName, nil)
	if err != nil {
		fmt.Printf("Error fetching repositories: %v", err)
		os.Exit(3)
	}
	for _, repo := range repos {
		if !includedRepos[repo.GetName()] {
			continue
		}
		err := updateLabelsInRepo(ctx, client, OrgName, repo, config.Labels)
		if err != nil {
			fmt.Printf("Error adjusting labels: %v", err)
			os.Exit(4)
		}
	}
}
