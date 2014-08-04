package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	contest      Contest
	templatePath *string
	staticPath   *string
	contestPath  *string
)

type Page struct {
	Title string
	Body  []byte
}

type Contest struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Problems  []Problem
}

type TempContest struct {
	Name      string
	StartTime string
	EndTime   string
	Problems  []Problem
}

type Problem struct {
	Name               string
	Difficulty         int
	ProblemDescription string
	InputFile          string
	OutputFile         string
	Generator          string
	URL                string
	Id                 int
}

/////////////////////////////////////////////////////////////////////////////
////////////////                  Pages                   ///////////////////
/////////////////////////////////////////////////////////////////////////////

//// Score Sheet ////
var scoreTemplate *template.Template

func openScoreSheet(w http.ResponseWriter, r *http.Request) {
	err := scoreTemplate.Execute(w, contest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//// Problem Page ////
var problemTemplate *template.Template

func newSubmissionDirectory() (string, error) {
	files, err := ioutil.ReadDir(*contestPath + "/submissions")
	if err != nil {
		fmt.Print("trouble reading directory", *contestPath+"/submissions", "\n")
		return "", errors.New("Something went wrong reading your file,\nPlease try again")
	}
	i := len(files)
	// TODO: zero-pad so that files sort correctly
	dirName := *contestPath + "/submissions/" + strconv.Itoa(i)
	_, err = os.Stat(dirName)
	// be careful to avoid overwriting anything
	for err == nil {
		i++
		dirName = *contestPath + "/submissions/" + strconv.Itoa(i) + "/"
		_, err = os.Stat(dirName)
	}
	return dirName, nil
}

func bail(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func openProblem(w http.ResponseWriter, r *http.Request, problemNum string) {
	i, err := strconv.Atoi(problemNum)
	if err != nil {
		fmt.Fprintf(w, "This problem does not exist")
		return
	}
	var problem Problem = contest.Problems[i]

	switch r.Method {
	case "GET": // displaying the problem
		//fileName := *contestPath + contest.Name + "/" + problem.Name
		//body, err := ioutil.ReadFile(fileName )
		//if err != nil{
		//	fmt.Print("Error loading problem ", fileName, "\n")
		//} else {
		err := problemTemplate.Execute(w, problem)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		//}
	case "POST":
		file, handler, err := r.FormFile("fileToUpload")
		if err != nil {
			fmt.Fprint(w, "Something went wrong while uploading,\nPlease try again")
			return
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Fprint(w, "Something went wrong reading your file,\nPlease try again")
			return
		}
		files, err := ioutil.ReadDir(*contestPath + "/submissions")
		if err != nil {
			fmt.Print("trouble reading directory", *contestPath+"/submissions", "\n")
			fmt.Fprint(w, "Something went wrong reading your file,\nPlease try again")
			return
		}
		dirName, err := newSubmissionDirectory()
		if err != nil {
			bail(w, err)
			return
		}
		newFileName := dirName + handler.Filename
		err = ioutil.WriteFile(newFileName, data, 0777)
		if err != nil {
			fmt.Fprint(w, "Something went wrong writing your file.\nPlease try again")
			return
		}
		fmt.Print("host = ", r.Host, "\n")
		//fmt.Fprint(w, r.Host + "/")

		addSubmission(newFileName, r.FormValue("user"), problem, handler.Filename)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

///// Judge Page ///////

var judgeTemplate *template.Template

func openJudge(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var contestBytes []byte
		var err error
		if len(contest.Name) == 0 {
			fakeContest := &TempContest{
				EndTime:   time.Now().Format(time.RFC3339),
				StartTime: time.Now().Format(time.RFC3339),
				Name:      "TEST",
				Problems:  make([]Problem, 6)}
			for i := 0; i < 6; i++ {
				fakeContest.Problems[i] = Problem{
					Name:       "one",
					Difficulty: i,
					InputFile:  "3.in",
					OutputFile: "3.out",
					Generator:  "eret",
					URL:        "www.googledriveurlhere.com"}
			}
			contestBytes, err = json.MarshalIndent(fakeContest, "", "    ")
		} else {
			contestBytes, err = json.MarshalIndent(contest, "", "    ")
		}
		if err != nil {
			fmt.Fprint(w, "Problem with contest format")
		} else {
			fmt.Print(string(contestBytes))
			err = judgeTemplate.Execute(w, string(contestBytes))
			if err != nil {
				fmt.Fprint(w, "Problem executing template")
				fmt.Fprint(w, err)
			}
		}
	case "POST":
		if r.FormValue("secret") != "ManBearPig" {
			fmt.Fprint(w, "Nuh uh uh you didn't say the magic word")
			return
		}

		fmt.Print("got here\n", r.FormValue("contest"), "\n")

		rawNewContest := []byte(r.FormValue("contest"))
		//dec := json.NewDecoder(strings.NewReader(r.FormValue("contest")))
		var cc TempContest
		err := json.Unmarshal(rawNewContest, &cc)
		//err := dec.Decode(&cc)
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Fprintf(w, "baddly formatted json: %v\n", err)
			fmt.Print("Error with json\n", err, "\n")
			return
		}

		// Convert temp to real
		contest.Name = cc.Name
		contest.Problems = cc.Problems
		contest.StartTime, err = time.Parse(time.RFC3339, cc.StartTime)
		contest.EndTime, err = time.Parse(time.RFC3339, cc.EndTime)

		// Load new files
		reader, err := r.MultipartReader()
		if err != nil {
			fmt.Fprint(w, "Something went wrong downloading,\nPlease try again")
			return
		}

		fmt.Print("Uplaoding files:\n")
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			fmt.Print(part.FileName, "\n")
			//if part.FileName() is empty, skip this iteration.
			if part.FileName() == "" {
				continue
			}
			dst, err := os.Create(*contestPath + contest.Name + "/" + part.FileName())
			defer dst.Close()

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if _, err := io.Copy(dst, part); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

//// Handle Submissions ////

type submission struct {
	User               string
	File               string
	SubmissionFileName string
	SubTime            time.Time
	Note               string
	Compiled           bool
	Ran                bool
	Correct            bool
	TimedOut           bool
	RunTime            time.Time
}

func addSubmission(file string, user string, problem Problem, subFileName string) *submission {
	newSub := &submission{File: file, User: user, SubTime: time.Now(),
		SubmissionFileName: subFileName}
	fileNameBase := strings.Split(filepath.Base(file), ".")[0]
	binFile := *contestPath + contest.Name + "/bin/" + fileNameBase
	switch filepath.Ext(file) {
	case ".c":
		//TODO
	case ".cpp":
		//TODO
	case ".js":
		//TODO
	case ".py":
		binFile = file
	case ".go":
		// compile file
		lser := exec.Command("echo", "go", "build", "-o", binFile, ".", file)
		testo, _ := lser.Output()
		fmt.Print("LKL", string(testo))
		builder := exec.Command("go", "build", "-o", binFile, file)
		buildText, err := builder.Output()
		fmt.Print("ghgh+", string(buildText), "\n")
		newSub.Note = string(buildText) + "\r\n"
		if err != nil {
			newSub.Note = newSub.Note + fmt.Sprint("Build failed error: %s \r\n", err)
			return newSub
		}

		// lint file
		//TODO
	case ".rb":
		binFile = file
	default:
		newSub.Note += "File type is not of a supported language"
		return newSub
	}

	newSub.Compiled = true

	//Test Program
	testCommand := ""
	if _, err := os.Stat(binFile); err == nil {
		switch filepath.Ext(file) {
		case ".py":
			testCommand = "python " + binFile
		case ".rb":
			testCommand = "ruby " + binFile
		case ".java":
			testCommand = "java " + binFile
		case ".c", ".cpp", ".go":
			testCommand = binFile
		}
	} else {
		newSub.Note = newSub.Note + "File did not produce file to run\r\n"
		return newSub
	}
	cmd := exec.Command("benchmark",
		fmt.Sprintf("%s%s/problems/%s", contestPath, contest.Name, problem.InputFile),
		fmt.Sprintf("%s%sbin/%s.out", contestPath, contest.Name, fileNameBase),
		testCommand)
	r, w, _ := os.Pipe()
	cmd.Stdout = w
	err := cmd.Start()
	if err != nil {
		fmt.Print("Error opening pipe:", err, "\n")
		return newSub
	}

	outC := make(chan string)

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(15 * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			log.Print("failed to kill: ", err)
			newSub.TimedOut = true
			w.Close()
			_ = <-outC
			return newSub
		}
		<-done // allow goroutine to exit
		log.Println("process killed")
	case err := <-done:
		if err != nil {
			log.Printf("process done with error = %v", err)
		}
	}

	w.Close()
	out_bytes := <-outC
	fmt.Printf("!!!%s\n", out_bytes)
	fmt.Println(len(out_bytes))

	newSub.Ran = true

	//Compare Output
	//diff -U 0 file1 file2 | grep -v ^@ | wc -l
	binComparer := exec.Command("countDiff",
		fmt.Sprintf("%s%sbin/%s.out", contestPath, contest.Name, fileNameBase),
		fmt.Sprintf("%s%s/problems/%s", contestPath, contest.Name, problem.OutputFile))
	binCompare, err := binComparer.Output()
	fmt.Print("Compare result: ", string(binCompare), "\n")
	differences, err := strconv.Atoi(strings.Trim(string(binCompare), "\n\r "))

	// 2 file lines to remove
	// only happen when differences are there
	if differences >= 2 {
		differences = differences - 2
	}
	if differences == 0 {
		newSub.Correct = true
		newSub.Note = newSub.Note + "Solution correct\n"
	} else {
		newSub.Note = newSub.Note + strings.Trim(string(binCompare), "\n\r ") + " differing lines\n"
	}
	if err != nil {
		newSub.Note = newSub.Note + fmt.Sprintf("Compare failed error: %s \r\n", err)
		return newSub
	}

	//TODO add big O

	return newSub
}

func serveStatic(w http.ResponseWriter, fileName string) {
	// open input file
	fi, err := os.Open(*staticPath + fileName)
	if err != nil {
		fmt.Print("error reading ", *staticPath+fileName, "\n")
		fmt.Fprint(w, "")
	}
	// close fi on exit and check for its returned error
	defer func() {
		if err := fi.Close(); err != nil {
			fmt.Print("error closing ", *staticPath+fileName, "\n")
		}
	}()
	// make a read buffer
	fr := bufio.NewReader(fi)
	io.Copy(w, fr)

}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	pathList := strings.Split(r.URL.Path, "/")
	fileName := pathList[len(pathList)-1]
	if len(pathList) == 0 {
		openScoreSheet(w, r)
	} else {
		switch filepath.Ext(fileName) {
		// serve static resources
		case ".js", ".html", ".ico", ".css":
			serveStatic(w, fileName)
		case ".json":
			fmt.Fprint(w, "TODO")
		default:
			if contest.Name != "" {
				if time.Time(contest.StartTime).Before(time.Now()) {
					if len(pathList) >= 3 && pathList[1] == "problem" {
						openProblem(w, r, pathList[2])
					} else if strings.Contains(r.URL.Path, "judge") {
						openJudge(w, r)
					} else {
						openScoreSheet(w, r)
					}
				} else {
					if strings.Contains(r.URL.Path, "judge") {
						openJudge(w, r)
					} else {
						fmt.Fprintf(w, "Contest will begin in: %v",
							time.Time(contest.StartTime).Sub(time.Now()))
					}
				}
			} else {
				if strings.Contains(r.URL.Path, "judge") {
					openJudge(w, r)
				} else {
					fmt.Fprint(w, "No Contest set up")
				}
			}
		}
	}
}

func loadContest() {
	contest.Name = ""
	// equivalent to Python's `if not os.path.exists(fileName)`
	if _, err := os.Stat(*contestPath + "contest.json"); os.IsNotExist(err) {
		log.Panicf("No saved contest in %s \n", *contestPath+"contest.json")
	} else {
		rawContest, err := ioutil.ReadFile(*contestPath + "contest.json")
		if err == nil {
			err = json.Unmarshal(rawContest, &contest)
			log.Printf("contest: %#v\n", contest)
		}

		if err != nil {
			log.Panicf("Problem loading contest.json, %#v\n", err)
		}
	}
}

func main() {
	contestPath = flag.String("contest", "contest/", "Where the contest files are located")
	templatePath = flag.String("templates", "templates/", "Where the template files are located")
	staticPath = flag.String("static", "static/", "Where the static site files are located")

	flag.Parse()
	fmt.Printf("contest path: %s\n", *contestPath)
	fmt.Printf("template path: %s\n", *templatePath)
	fmt.Printf("static path: %s\n", *staticPath)

	problemTemplate = template.Must(template.ParseFiles(*templatePath + "problem.html"))
	scoreTemplate = template.Must(template.ParseFiles(*templatePath + "score.html"))
	judgeTemplate = template.Must(template.ParseFiles(*templatePath + "judge.html"))

	loadContest()

	http.HandleFunc("/", mainHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
