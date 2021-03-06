package source

import (
	"fmt"
	"log"
	"strings"

	copier "github.com/otiai10/copy"
)

type Paths []string

// copies files or directories into workdir/id/<name>
func (paths Paths) Copy(workDir, orderID string) error {
	log.Println("Copying from path...")
	for _, path := range paths {
		parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
		name := parts[len(parts)-1]
		err := copier.Copy(path, fmt.Sprintf("%s/orders/%s/%s", workDir, orderID, name))
		if err != nil {
			return err
		}
	}
	return nil
}
