package main

import (
	"flag"
	"fmt"
	"github.com/xlab/treeprint"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var pathFlag = flag.String("path", "", "put your full alembic migrations path here, like ./alembic/versions")

func main() {

	flag.Parse()

	if pathFlag == nil || *pathFlag == "" {
		panic("path is empty")
	}

	path := *pathFlag

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	files, err := ioutil.ReadDir(*pathFlag)
	if err != nil {
		log.Fatal(err)
	}

	var migrations []Migration

	for _, f := range files {
		migrations = append(migrations, parseHeader(*pathFlag, f))
	}

	mtree := buildTree(migrations)

	tree := convertToTreeprint(mtree)
	fmt.Println(tree.String())
}

type Migration struct {
	Name       string
	RevisionID string
	Revises    *string
	Date       string
}

type MigrationTree struct {
	M        Migration
	Parent   *MigrationTree
	Children []*MigrationTree
}

func getTillNL(c string) (string, int) {
	id := 0
	for i := range c {
		if c[i] == '\n' {
			id = i
			break
		}
	}

	return c[:id], id
}

func getAfterLastSpace(c string) string {
	b := []byte(c)
	id := 0
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == ' ' {
			id = i
			break
		}
	}

	return c[id+1:]
}

func parseHeader(path string, file fs.FileInfo) Migration {
	contentBytes, err := os.ReadFile(path + file.Name())
	if err != nil {
		panic(err)
	}

	content := string(contentBytes)
	content = content[3:]

	name, nextId := getTillNL(content)

	content = content[nextId+2:]

	revisionIDStr, nextId := getTillNL(content)

	revisionID := getAfterLastSpace(revisionIDStr)

	content = content[nextId+1:]

	revisesStr, nextId := getTillNL(content)

	revises := getAfterLastSpace(revisesStr)

	content = content[nextId+1:]

	dateStr, nextId := getTillNL(content)

	date := getAfterLastSpace(dateStr)

	return Migration{
		Name:       name,
		RevisionID: revisionID,
		Revises:    &revises,
		Date:       date,
	}
}

func buildTreeRec(parent *MigrationTree, others []Migration) *MigrationTree {
	for i := range others {
		if *others[i].Revises == parent.M.RevisionID {
			child := &MigrationTree{
				M:        others[i],
				Parent:   parent,
				Children: nil,
			}
			parent.Children = append(parent.Children, child)
			others = append(others[:i], others[i+1:]...)
			return buildTreeRec(child, others)
		}
	}

	if len(others) == 0 {
		return parent
	}

	return buildTreeRec(parent.Parent, others)
}

func buildTree(m []Migration) *MigrationTree {
	// let's find root
	var root *MigrationTree
	for i := range m {
		if m[i].Revises == nil || *m[i].Revises == "" {
			root = &MigrationTree{
				M:        m[i],
				Parent:   nil,
				Children: nil,
			}

			m = append(m[:i], m[i+1:]...)

			break
		}
	}

	if root == nil {
		panic("no root found")
	}

	buildTreeRec(root, m)

	return root
}

func convertToTreeprintRec(root *MigrationTree, tree treeprint.Tree) {
	for i := range root.Children {
		convertToTreeprintRec(root.Children[i], tree.AddMetaBranch(root.Children[i].M.RevisionID, root.Children[i].M.Name))
	}
}

func convertToTreeprint(root *MigrationTree) treeprint.Tree {
	tree := treeprint.New()

	convertToTreeprintRec(root, tree.AddMetaBranch(root.M.RevisionID, root.M.Name))

	return tree
}
