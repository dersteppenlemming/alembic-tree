package main

import (
	"flag"
	"github.com/xlab/treeprint"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

var pathFlag = flag.String("path", "", "put your full alembic migrations path here, like ./alembic/versions")

func main() {

	flag.Parse()

	if pathFlag == nil || *pathFlag == "" {
		log.Fatal("path flag is empty")
	}

	path := *pathFlag

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatalf("error reading dir: %v", err)
	}

	var migrationsByHeader, migrationsActual []Migration

	for _, f := range files {
		if !f.IsDir() {
			mh, ma := parse(path, f)
			migrationsByHeader = append(migrationsByHeader, mh)
			migrationsActual = append(migrationsActual, ma)
		}
	}

	mTreeHeader := buildTree(migrationsByHeader, "header")

	mTreeActual := buildTree(migrationsActual, "actual")

	headerTreeStr := convertToTreeprint(mTreeHeader).String()

	actualTreeStr := convertToTreeprint(mTreeActual).String()

	log.Println(buildPrintString(headerTreeStr, actualTreeStr))

	if headerTreeStr != actualTreeStr {
		log.Fatalf("Trees are not equal")
	} else {
		log.Println("Trees are equal")
	}
}

func buildPrintString(hs, as string) string {
	var s strings.Builder

	s.WriteString("\n--------------------HEADER-TREE-------------------\n")
	s.WriteString(hs)
	s.WriteString("\n--------------------ACTUAL-TREE-------------------\n")
	s.WriteString(as)
	s.WriteString("\n--------------------------------------------------\n")

	return s.String()
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

var (
	nameExp           = regexp.MustCompile(`^"""(.+)\n`)
	revisionHeaderExp = regexp.MustCompile(`Revision ID:\ *(.+)\n`)
	revisesExp        = regexp.MustCompile(`Revises:\ *(.+)\n`)
	dateExp           = regexp.MustCompile(`Create Date:\ *(.+)\n`)
)

func parseHeader(filename, content string) Migration {
	names := nameExp.FindStringSubmatch(content)

	var name string
	if len(names) == 2 {
		name = names[1]
	}

	revisions := revisionHeaderExp.FindStringSubmatch(content)
	if revisions == nil || len(revisions) == 0 || len(revisions) != 2 {
		log.Fatalf("no revision found or more than one in {%s}: %v", filename, revisions)
	}

	downRevisions := revisesExp.FindStringSubmatch(content)
	if len(downRevisions) > 2 {
		log.Fatalf("more than one down_revision found in {%s}: %v", filename, downRevisions)
	}

	var downRevision *string
	if len(downRevisions) == 2 && strings.Trim(downRevisions[1], " \n\t") != "" {
		downRevision = &downRevisions[1]
	}

	dates := dateExp.FindStringSubmatch(content)
	if dates == nil || len(dates) == 0 || len(dates) != 2 {
		log.Fatalf("no date found or more than one in {%s}: %v", filename, dates)
	}

	return Migration{
		Name:       name,
		RevisionID: revisions[1],
		Revises:    downRevision,
		Date:       dates[1],
	}
}

var revisionExp = regexp.MustCompile(`revision\ *=\ *"(.+)"`)

var downExp = regexp.MustCompile(`down_revision\ *=\ *"(.+)"`)

func parseActual(filename, content, name, date string) Migration {
	revisions := revisionExp.FindStringSubmatch(content)
	if revisions == nil || len(revisions) == 0 || len(revisions) != 2 {
		log.Fatalf("no revision found or more than one in {%s}: %v", filename, revisions)
	}

	downRevisions := downExp.FindStringSubmatch(content)
	if len(downRevisions) > 2 {
		log.Fatalf("more than one down_revision found in {%s}: %v", filename, downRevisions)
	}

	var downRevision *string
	if len(downRevisions) == 2 {
		downRevision = &downRevisions[1]
	}

	return Migration{
		Name:       name,
		RevisionID: revisions[1],
		Revises:    downRevision,
		Date:       date,
	}
}

func parse(path string, file fs.FileInfo) (header, actual Migration) {
	contentBytes, err := os.ReadFile(path + file.Name())
	if err != nil {
		log.Fatalf("error reading file: %v", err)
	}

	content := string(contentBytes)

	header = parseHeader(file.Name(), content)

	actual = parseActual(file.Name(), content, header.Name, header.Date)

	return header, actual
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

func buildTree(m []Migration, parseType string) *MigrationTree {
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
		log.Fatalf("no root found, parse type: %v", parseType)
	}

	buildTreeRec(root, m)

	return root
}

func convertToTreeprintRec(root *MigrationTree, tree treeprint.Tree) {
	if len(root.Children) == 0 {
		return
	}

	if len(root.Children) == 1 {
		convertToTreeprintRec(root.Children[0], tree.AddMetaNode(root.Children[0].M.RevisionID, root.Children[0].M.Name))

		return
	}

	for i := range root.Children {
		convertToTreeprintRec(root.Children[i], tree.FindLastNode().AddMetaBranch(root.Children[i].M.RevisionID, root.Children[i].M.Name))
	}
}

func convertToTreeprint(root *MigrationTree) treeprint.Tree {
	tree := treeprint.New()

	convertToTreeprintRec(root, tree.AddMetaNode(root.M.RevisionID, root.M.Name))

	return tree
}
