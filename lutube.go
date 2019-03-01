package main

import (
  "os"
  "io"
  "io/ioutil"
  "net/http"
  "net/url"
  "html/template"
  "log"
)

// Basic structure for storing the important information about a video.
type Video struct {
  Id string
  Title string
}

// Given a video ID, creates a Video object by fetching the relevant
// information from the file system.
func loadVideo(id string) (*Video, error) {
  filename := "videos/" + id + "/videodata.txt"
  videoData, err := ioutil.ReadFile(filename)
  if err != nil {
    return nil, err
  }
  title := string(videoData)
  return &Video{Id: id, Title: title}, nil
}

// Given a video information structure and a video file, saves the video
// information and the file to disc.
func saveVideo(video *Video, videoFile io.Reader) error {
  // Create the video file on the server and copies the network file to it.
  videoDirectory := "./videos/" + video.Id
  serverVideoFile, err := os.Create(videoDirectory + "/video.mp4")
  if err != nil {
    return err
  }
  defer serverVideoFile.Close()

  _, err1 := io.Copy(serverVideoFile, videoFile)
  if err1 != nil {
    return err
  }

  // Store the title of the video into the 'videodata.txt' file.
  videoDataFile, err2 := os.Create(videoDirectory + "/videodata.txt")
  if err2 != nil {
    return err
  }
  defer videoDataFile.Close()

  _, err3 := videoDataFile.WriteString(video.Title)
  return err3
}

// Takes an HTML template and a collection of data used to populate it, and
// renders the template, broadcasting it to a given HTTP response writer.
func renderTemplate(writer http.ResponseWriter, templateFile string, data interface{}) {
  templ, err := template.ParseFiles(templateFile + ".html")
  if err != nil {
    http.Error(writer, err.Error(), http.StatusInternalServerError)
  }
  templ.Execute(writer, data)
}

// HTTP handler used for watching videos. Loads the video from its ID and
// uses it to create the HTTP response page.
func watchHandler(writer http.ResponseWriter, request *http.Request) {
  id := request.URL.Path[len("/watch/"):]
  video, err := loadVideo(id)
  if err != nil {
    http.Redirect(writer, request, "/?error=notfound&id=" + id, http.StatusSeeOther)
  }
  renderTemplate(writer, "watch", video)
}

// Gets the list of all videos stored on the file system.
func getAvailableVideos() ([]*Video, error) {
  videoDirectories, err := ioutil.ReadDir("videos")
  if err != nil {
    return nil, err
  }
  availableVideos := make([]*Video, 0)
  for _, f := range videoDirectories {
    video, _ := loadVideo(f.Name())
    availableVideos = append(availableVideos, video)
  }
  return availableVideos, nil
}

// Decodes error codes from URL queries into messages displayed on the page.
func getErrorMessage(query url.Values) string {
  errorType := query.Get("error")
  switch (errorType) {
  case "notfound":
    return "ID " + query.Get("id") + " not found."
  case "misc":
    return "Error: " + query.Get("msg")
  }
  return ""
}

// HTTP handler used for rendering the home page. Displays a list of all the
// videos living on the file system.
func homeHandler(writer http.ResponseWriter, request *http.Request) error {
  videoList, err := getAvailableVideos()
  if err != nil {
    return err
  }
  errorMessage := getErrorMessage(request.URL.Query())

  data := struct {
    VideoList []*Video
    ErrorMessage string
  }{
    videoList,
    errorMessage,
  }
  renderTemplate(writer, "home", data)
  return nil
}

// Handles POST requests for the uploading of videos. Uploads the video to
// the server along with its title.
func uploadHandler(writer http.ResponseWriter, request *http.Request) error {
  // Parse the request and extract the video and title from it.
  err := request.ParseMultipartForm(64 << 20)
  if err != nil {
    return err
  }

  videoFile, _, err1 := request.FormFile("video-file")
  if err1 != nil {
    return err1
  }
  defer videoFile.Close()
  title := request.FormValue("title")

  // The TempDir function creates a unique subdirectory of a given directory.
  // Use this to generate a unique ID for the new video.
  videoDir, err2 := ioutil.TempDir("videos/", "")
  if err2 != nil {
    return err2
  }
  id := videoDir[len("./video"):]

  // Save the video to the server and redirect the user to the new page where
  // they can watch it.
  err3 := saveVideo(&Video{Id: id, Title: title}, videoFile)
  if err3 != nil {
    http.Redirect(writer, request, "/?error=fu", http.StatusSeeOther)
    return nil
  }

  http.Redirect(writer, request, "/watch/" + id, http.StatusSeeOther)
  return nil
}

// A function that deals with HTTP requests and returns an error. Should
// usually be converted to a function that returns nothing using an error
// handler and then passed to http.HandleFunc().
type appHandler func(http.ResponseWriter, *http.Request) error

func (fn appHandler) internalServerErrorHandler(writer http.ResponseWriter, request *http.Request) {
  err := fn(writer, request)
  if err != nil {
    http.Error(writer, err.Error(), http.StatusInternalServerError)
    panic(err)
    return
  }
}

// Handles errors by redirecting the user to the home page and displaying the
// error message on it.
func (fn appHandler) homePageHandler(writer http.ResponseWriter, request *http.Request) {
  err := fn(writer, request)
  if err != nil {
    http.Redirect(writer, request, "/?error=misc&msg=" + err.Error(), http.StatusSeeOther)
    return
  }
}

// Sets up the handlers and serves on to port 8080.
func main() {
  http.HandleFunc("/watch/", watchHandler)
  http.Handle("/videos/", http.FileServer(http.Dir(".")))
  http.HandleFunc("/upload/", appHandler(uploadHandler).homePageHandler)
  http.HandleFunc("/", appHandler(homeHandler).internalServerErrorHandler)
  log.Fatal(http.ListenAndServe(":8080", nil))
}
