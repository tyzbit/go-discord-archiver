# go-archive
Go package for interacting with archive.org

## Usage

```golang
package main

import "github.com/tyzbit/go-archive"

func main() {
    requestUrls := []string{"https://example.com", "https://archive.org"}
    urls := archiveorg.GetLatestArchiveURLs(requestUrls, true)

    for url := range urls {
        fmt.Println("url: ", url)
    }
}
```