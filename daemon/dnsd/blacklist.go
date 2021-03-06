package dnsd

import (
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"strings"
	"sync"
)

// HostsFileURLs is a collection of URLs where up-to-date ad/malware/spyware blacklist hosts files are published.
var HostsFileURLs = []string{
	"http://winhelp2002.mvps.org/hosts.txt",
	"http://pgl.yoyo.org/adservers/serverlist.php?hostformat=hosts&showintro=0&mimetype=plaintext",
	"http://www.malwaredomainlist.com/hostslist/hosts.txt",
	"http://someonewhocares.org/hosts/hosts",
}

// DownloadAllBlacklists attempts to download all hosts files and return combined list of domain names to block.
func DownloadAllBlacklists(logger misc.Logger) []string {
	wg := new(sync.WaitGroup)
	wg.Add(len(HostsFileURLs))

	// Download all lists in parallel
	lists := make([][]string, len(HostsFileURLs))
	for i, url := range HostsFileURLs {
		go func(i int, url string) {
			resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: BlacklistDownloadTimeoutSec}, url)
			names := ExtractNamesFromHostsContent(string(resp.Body))
			logger.Info("DownloadAllBlacklists", url, err, "downloaded %d names, please obey the license in which the list author publishes the data.", len(names))
			lists[i] = ExtractNamesFromHostsContent(string(resp.Body))
			defer wg.Done()
		}(i, url)
	}
	wg.Wait()
	ret := UniqueStrings(lists...)
	logger.Info("DownloadAllBlacklists", "", nil, "downloaded %d unique names in total", len(ret))
	return ret
}

/*
ExtractNamesFromHostsContent extracts domain names from hosts file content. It will understand and skip comments and
empty lines.
*/
func ExtractNamesFromHostsContent(content string) []string {
	ret := make([]string, 0, 16384)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			// Skip blank and comments
			continue
		}
		// Find the second field
		space := strings.IndexRune(line, ' ')
		if space == -1 {
			// Skip malformed line
			continue
		}
		line = strings.TrimSpace(line[space:])
		nameEnd := strings.IndexRune(line, '#')
		// Name may be followed by a comment
		if nameEnd == -1 {
			nameEnd = len(line)
		}
		// Extract the name itself
		aName := strings.ToLower(strings.TrimSpace(line[:nameEnd]))
		if aName == "" || strings.HasSuffix(aName, "localhost") || strings.HasSuffix(aName, "localdomain") || len(aName) < 4 {
			// Skip empty names, local names, and overly short names
			continue
		}
		ret = append(ret, aName)
	}
	return ret
}

// UniqueStrings returns unique strings among input string arrays.
func UniqueStrings(arrays ...[]string) []string {
	m := map[string]struct{}{}
	for _, array := range arrays {
		if array == nil {
			continue
		}
		for _, str := range array {
			m[str] = struct{}{}
		}
	}
	ret := make([]string, 0, len(m))
	for str := range m {
		ret = append(ret, str)
	}
	return ret
}
