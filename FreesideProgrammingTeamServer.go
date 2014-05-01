package main

import (
	"fmt"
	"flag"
	"os"
	"io"
	"os/exec"
	"html/template"
	"io/ioutil"
	"log"
//	"net"
	"net/http"
//	"regexp"
	"time"
	"strings"
	"strconv"
	"path/filepath"
	"bufio"
	"net/smtp"
	"bytes"
	"encoding/json"
)

var (
	contest Competition
	hostPath *string
	contestPath *string

	emailUser = &EmailUser{"freesideprogramming", "freesidepassword", "smtp.gmail.com", 587}

	auth = smtp.PlainAuth("",
		emailUser.Username,
		emailUser.Password,
		emailUser.EmailServer)
)



type EmailUser struct {
	Username    string
	Password    string
	EmailServer string
	Port        int
}

type Page struct {
	Title string
	Body  []byte
}

type Competition struct {
	Name string
	StartTime time.Time
	EndTime time.Time
	Problems []Problem
}

type TempCompetition struct {
	Name string
	StartTime string
	EndTime string
	Problems []Problem
}

type Problem struct {
	Name string
	Difficulty int
	InputFile string
	OutputFile string
	Generator string
	URL string
}

type jsonTime time.Time

func (t jsonTime) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(time.Time(t).Format(time.RFC3339))), nil
}

func (t *jsonTime) UnmarshalJSON(s []byte) (err error) {
	q, err := strconv.Unquote(string(s))
	if err != nil {
		return err
	}
	*(*time.Time)(t), err = time.Parse(time.RFC3339, q)
	return
}

func (t jsonTime) String() string	{ return time.Time(t).String() }

/*func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}*/

/*func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}*/

///*func mainHandler(w http.ResponseWriter, r *http.Request, title string) {
//
//	renderTemplate(w, "homebefore", thisCompetition)
//	renderTemplate(w, "homeon", thisCompetition)
//
//}*/

/////////////////////////////////////////////////////////////////////////////
////////////////                  Pages                   ///////////////////
/////////////////////////////////////////////////////////////////////////////

//// Score Sheet ////
var scoreTemplate *template.Template

func openScoreSheet(w http.ResponseWriter, r *http.Request) {
	err := scoreTemplate.Execute(w, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//// Problem Page ////
var problemTemplate *template.Template

type problemTemplateFormatter struct {
	Name string
	URL string
	Difficulty int
}

func openProblem(w http.ResponseWriter, r *http.Request, prob string) {
	var probVar Problem
	isFound := false
	for _,problem := range contest.Problems {
		if prob == problem.Name {
			probVar = problem
			isFound = true
			break
		}
	}
	if !isFound {
		fmt.Fprintf(w, "This problem does not exsist")
		return
	}
	switch r.Method {
	case "GET":
		//fileName := *contestPath + contest.Name + "/" + probVar.Name
		//body, err := ioutil.ReadFile(fileName )
		//if err != nil{
		//	fmt.Print("Error loading problem ", fileName, "\n")
		//} else {
			p := &problemTemplateFormatter{Name: probVar.Name, URL: probVar.URL , Difficulty: probVar.Difficulty}
			err := problemTemplate.Execute(w, p)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		//}
	case "POST":
		file, handler, err := r.FormFile("fileToUpload")
		if err != nil {
			fmt.Fprint(w, "Something went wrong downloading,\nPlease try again")
			return
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Fprint(w, "Something went wrong reading your file,\nPlease try again")
			return
		}
		files, err := ioutil.ReadDir(*contestPath + contest.Name + "/subs")
		if err != nil {
			fmt.Print("trouble reading directory", *contestPath + contest.Name + "/subs", "\n")
			fmt.Fprint(w, "Something went wrong reading your file,\nPlease try again")
			return
		}
		newFileName := *contestPath + contest.Name + "subs/" + strconv.Itoa(len(files)) + filepath.Ext(handler.Filename)
		err = ioutil.WriteFile(newFileName, data, 0777)
		if err != nil {
			fmt.Fprint(w, "Something went wrong writing your file.\nPlease try again")
			return
		}
		fmt.Print("host = ", r.Host, "\n")
		//fmt.Fprint(w, r.Host + "/")

		sub := addSubmission(newFileName, r.FormValue("user"), r.FormValue("email"), probVar, handler.Filename)

		emailSubmission(sub)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}


var judgeTemplate *template.Template

func openJudge(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var contestBytes []byte
		var err error
		if len(contest.Name) == 0 {
			fakeContest := &TempCompetition{
				EndTime: time.Now().Format(time.RFC3339Nano),
				StartTime: time.Now().Format(time.RFC3339Nano),
				Name: "TEST",
				Problems: make([]Problem, 6)}
			for i := 0; i < 6; i++ {
				fakeContest.Problems[i] = Problem{
					Name: "one",
					Difficulty: i,
					InputFile: "3.in",
					OutputFile: "3.out",
					Generator: "eret",
					URL: "www.googledriveurlhere.com"}
			}
			contestBytes, err = json.MarshalIndent(fakeContest, "", "    ")
		} else {
			contestBytes, err = json.MarshalIndent(contest, "", "    ")
		}
		if err != nil {
			fmt.Fprint(w,"Problem with contest format")
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

		fmt.Print("got here\n",r.FormValue("contest"),"\n")

		rawNewContest := []byte(r.FormValue("contest"))
		//dec := json.NewDecoder(strings.NewReader(r.FormValue("contest")))
		var cc TempCompetition
		err := json.Unmarshal(rawNewContest, &cc)
		//err := dec.Decode(&cc)
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Fprintf(w, "baddly formatted json: %v\n", err)
			fmt.Print("Error with json\n",err,"\n")
			return
		}


		// Contert temp to real
		contest.Name = cc.Name
		contest.Problems = cc.Problems
		contest.StartTime, err = time.Parse(time.RFC3339Nano, cc.StartTime)
		contest.EndTime, err = time.Parse(time.RFC3339Nano, cc.EndTime)

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
			fmt.Print(part.FileName,"\n")
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
	User string
	Email string
	File string
	SubmissionFileName string
	SubTime time.Time
	Note string
	Compiled bool
	Ran bool
	Correct bool
	TimedOut bool
	RunTime time.Time
}

func addSubmission(file string, user string, email string, probVar Problem, subFileName string) *submission{
	newSub := &submission{File: file, User: user, Email: email, SubTime: time.Now(),
			SubmissionFileName: subFileName}
	fileNameBase := strings.Split(filepath.Base(file),".")[0]
	binFile := *contestPath + contest.Name + "/bin/" + fileNameBase
	switch filepath.Ext(file){
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
		fmt.Print("LKL",string(testo))
		builder := exec.Command( "go", "build", "-o", binFile, file)
		buildText, err := builder.Output()
		fmt.Print("ghgh+",string(buildText),"\n")
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
		newSub.Note += "File type is not of a suported language"
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
		fmt.Sprintf("%s%s/problems/%s", contestPath, contest.Name, probVar.InputFile),
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
	            if err!=nil{
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
		fmt.Sprintf("%s%sbin/%s.out",contestPath, contest.Name, fileNameBase),
		fmt.Sprintf("%s%s/problems/%s",contestPath,contest.Name, probVar.OutputFile))
	binCompare, err := binComparer.Output()
	fmt.Print("Compare result: ",string(binCompare), "\n")
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


func emailSubmission(toSend *submission) {
	text := fmt.Sprintf("Subject: Submission %s\r\n\r\nUser: %s\r\nLanguage: %s\r\nSubmit Time: %s\r\nCompiled: %t\r\nRan: %t\r\nTimedOut: %t\r\nCorrect: %t\r\nNote: %s",
		filepath.Base(toSend.File),
		toSend.User,
		filepath.Ext(toSend.File),
		toSend.SubTime,
		toSend.Compiled,
		toSend.Ran,
		toSend.TimedOut,
		toSend.Correct,
		toSend.Note)
	err := smtp.SendMail(emailUser.EmailServer+":"+strconv.Itoa(emailUser.Port), // in our case, "smtp.google.com:587"
		auth,
		emailUser.Username,
		[]string{toSend.Email},
		[]byte(text))
	fmt.Print(text)
	if err != nil {
		log.Print("ERROR: attempting to send a mail ", err)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%+v\n", r)
	pathList := strings.Split(r.URL.Path, "/")
	endPath := pathList[len(pathList) - 1]
	if len(pathList) == 0 {
		openScoreSheet(w, r)
	} else {
		switch filepath.Ext(endPath) {
		case ".js", ".html", ".ico", ".css" :
			// open input file
			fi, err := os.Open(*hostPath + endPath)
			if err != nil {
				fmt.Print("error reading ",  *hostPath + endPath, "\n")
				fmt.Fprint(w,"")
			}
			// close fi on exit and check for its returned error
			defer func() {
				if err := fi.Close(); err != nil {
					fmt.Print("error closing ",  *hostPath + endPath, "\n")
				}
			}()
			// make a read buffer
			fr := bufio.NewReader(fi)
			io.Copy(w,fr)
		case ".json":
			fmt.Fprint(w,"TODO")
		default:
			if contest.Name != "" {
				if time.Time(contest.StartTime).Before(time.Now()) {
					if len(pathList) >= 3 && pathList[1] == "problem" {
						openProblem(w, r, pathList[2])
					} else if strings.Contains(r.URL.Path, "judg") {
						openJudge(w, r)
					} else {
						openScoreSheet(w,r)
					}
				} else {
					if strings.Contains(r.URL.Path, "judg") {
						openJudge(w, r)
					} else {
						fmt.Fprintf(w, "Contest will begin in: %v",
							time.Time(contest.StartTime).Sub(time.Now()))
					}
				}
			} else {
				if strings.Contains(r.URL.Path, "judg") {
					openJudge(w, r)
				} else {
					fmt.Fprint(w, "No Competition set up")
				}
			}
		}
	}
}


//func renderTemplate(w http.ResponseWriter, tmpl string, p interface{}) {
/*func renderTemplate(w http.ResponseWriter, tmpl string, p string) {
	err := scoreTemplate.Execute(w, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}*/

/*var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		fn(w, r, m[2])
	}
}*/

//TODO reomve
///////////////////////////////////////////////////////////////////////
////                    File host for css/images/etc               ////
///////////////////////////////////////////////////////////////////////
/*func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
    w.WriteHeader(status)
    if status == http.StatusNotFound {
	http.Redirect(w, r, "/", http.StatusFound)
    }
}

type justFilesFilesystem struct {
    fs http.FileSystem
}

func (fs justFilesFilesystem) Open(name string) (http.File, error) {
    f, err := fs.fs.Open(name)
    if err != nil {
        return nil, err
    }
    return neuteredReaddirFile{f}, nil
}

type neuteredReaddirFile struct {
    http.File
}

func (f neuteredReaddirFile) Readdir(count int) ([]os.FileInfo, error) {
    return nil, nil
}*/


/////////////////////////////////////////////////////////////////////////
func main() {
	contest.Name = ""
	hostPath = flag.String("host", "resources/", "Where the site files are located")
	contestPath = flag.String("contest", "contests/", "Where the contest files are located")

	flag.Parse()


	problemTemplate = template.Must(template.ParseFiles(*hostPath + "problem.html"))
	scoreTemplate = template.Must(template.ParseFiles(*hostPath + "score.html"))
	judgeTemplate = template.Must(template.ParseFiles(*hostPath + "judge.html"))

	// equivalent to Python's `if not os.path.exists(filename)`
	if _, err := os.Stat(*contestPath + "contest.json"); os.IsNotExist(err) {
		fmt.Printf("no saved contest")
	} else {
		rawContest, err := ioutil.ReadFile(*contestPath + "contest.json")
		if err != nil {
			err = json.Unmarshal(rawContest, &contest)
		}

		if err != nil {
			fmt.Print("Problem loading contest.json\n")
		}
	}

	fmt.Print(*hostPath, "\n")
	fmt.Print(*contestPath, "\n")
	fmt.Print(contest.Name, "\n")
	err := os.Chdir(*contestPath)
	if err != nil {
		log.Fatal(err)
		return
	}
	http.HandleFunc("/", mainHandler)
	http.ListenAndServe(":80", nil)
	//cgi.Serve(nil)
}
