package github

import "strings"

func (gc *GithubClient) getRepoNameFromURL(url string) (string, string) {
	// https://github.com/octocat/Hello-World/pull/1347
	url = strings.ReplaceAll(url, "https://github.com/", "")
	pieces := strings.Split(url, "/")
	return pieces[0], pieces[1]
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
