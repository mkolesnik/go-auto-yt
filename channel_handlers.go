package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

func HandleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, "log.log")
}

func HandlePlaylists(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/playlists.html")
}

func HandleVideos(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/videos.html")
}

func HandleAddChannel(w http.ResponseWriter, r *http.Request) {
	log.Info("received a request to add a channel")
	var channelData AddChannelPayload
	var res Response
	err := json.NewDecoder(r.Body).Decode(&channelData)
	if err != nil {
		log.Error("HandleAddChannel: ", err)
		res = Response{Type: "Error", Key: "ERROR_PARSING_DATA", Message: "There was an error parsing json: " + err.Error()}
	}
	log.Info(channelData)

	channel := DownloadTarget{URL: channelData.URL, Type: "Channel"}

	log.Info("CHECKING IF CHANNEL ALREADY EXISTS")
	doesChannelExist, err := channel.DoesExist()
	if err != nil {
		log.Info("error doesChannelExist: ", err)
		res = Response{Type: "Error", Key: "DOES_EXIST_ERROR", Message: "There was an error while trying to check if the channel already exists" + err.Error()}
	}
	if doesChannelExist == true {
		log.Info("this channel already exists")
		res = Response{Type: "Success", Key: "CHANNEL_ALREADY_EXISTS", Message: "This channel already exists"}
	} else {
		log.Info("channel doesn't exist")
		channelMetadata, err := channel.GetMetadata()
		if err != nil {
			res = Response{Type: "Error", Key: "ERROR_GETTING_METADATA", Message: "There was an error getting channel metadata: " + err.Error()}
		}

		if channelData.DownloadMode == "Audio Only" {
			channel = DownloadTarget{URL: channelData.URL, DownloadMode: channelData.DownloadMode, Name: channelMetadata.Uploader, PreferredExtensionForAudio: channelData.FileExtension, DownloadHistory: []string{}, LastChecked: time.Now().Format("01-02-2006 15:04:05"), CheckingInterval: "", Type: "Channel"}
		}	else if channelData.DownloadMode == "Video And Audio" {
			channel = DownloadTarget{URL: channelData.URL, DownloadMode: channelData.DownloadMode, Name: channelMetadata.Uploader, PreferredExtensionForVideo: channelData.FileExtension, DownloadHistory: []string{}, LastChecked: time.Now().Format("01-02-2006 15:04:05"), CheckingInterval: "", Type: "Channel"}
		}

		err = channel.AddToDatabase()
		if err != nil {
			log.Error(err)
			res = Response{Type: "Error", Key: "ERROR_ADDING_CHANNEL", Message: "There was an error adding the channel to the database" + err.Error()}
		}

		err = channel.Download(channelData.DownloadQuality, channelData.FileExtension, channelData.DownloadEntire)
		if err != nil {
			res = Response{Type: "Error", Key: "ERROR_DOWNLOADING_ENTIRE_CHANNEL", Message: "There was an error downloading the entire channel" + err.Error()}
			ReturnResponse(w, res)
		}
		err = channel.UpdateLatestDownloaded(channelMetadata.ID)
		if err != nil {
			res = Response{Type: "Error", Key: "ERROR_CHECKING_CHANNEL", Message: "There was an error while checking the channel: " + err.Error()}
			ReturnResponse(w, res)
		}
		err = channel.UpdateDownloadHistory(channelMetadata.ID)
		if err != nil {
			res = Response{Type: "Error", Key: "ERROR_CHECKING_CHANNEL", Message: "There was an error while checking the channel: " + err.Error()}
			ReturnResponse(w, res)
		}
		res = Response{Type: "Success", Key: "ADD_CHANNEL_SUCCESS", Message: "Channel successfully added and downloaded latest video"}
		ReturnResponse(w, res)
	}
}

func HandleCheckChannel(w http.ResponseWriter, r *http.Request) {
	log.Info("received a request to check a channel for new uploads")
	w.Header().Set("Content-Type", "application/json")
	var data AddChannelPayload
	var res Response
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		ReturnResponse(w, Response{Type: "Error", Key: "ERROR_PARSING_DATA", Message: "There was an error parsing json: " + err.Error()})
	}
	channel := DownloadTarget{URL: data.URL, Type: "Channel"}
	channel, _ = channel.GetFromDatabase()
	newVideoFound, videoId, err := channel.CheckNow()
	if err != nil {
		res = Response{Type: "Error", Key: "ERROR_CHECKING_CHANNEL", Message: "There was an error while checking the channel: " + err.Error()}
	}
	if newVideoFound == true {
		err = channel.Download("best", data.FileExtension, false)
		if err != nil {
			res = Response{Type: "Error", Key: "ERROR_CHECKING_CHANNEL", Message: "There was an error while checking the channel: " + err.Error()}
		}
		err = channel.UpdateLatestDownloaded(videoId)
		if err != nil {
			res = Response{Type: "Error", Key: "ERROR_CHECKING_CHANNEL", Message: "There was an error while checking the channel: " + err.Error()}
		}
		err = channel.UpdateDownloadHistory(videoId)
		if err != nil {
			res = Response{Type: "Error", Key: "ERROR_CHECKING_CHANNEL", Message: "There was an error while checking the channel: " + err.Error()}
		}
		res = Response{Type: "Success", Key: "NEW_VIDEO_DETECTED", Message: "New video detected for " + channel.Name + " and downloaded"}
	} else {
		res = Response{Type: "Success", Key: "NO_NEW_VIDEOS", Message: "No new videos detected for " + channel.Name}
	}
	ReturnResponse(w, res)
}

func HandleCheckAll(w http.ResponseWriter, r *http.Request) {
	log.Info("received a request to check all channels for new uploads")
	w.Header().Set("Content-Type", "application/json")
	res, err := CheckAll("channels")
	if err != nil {
		ReturnResponse(w, Response{Type: "Error", Key: "ERROR_CHECKING_CHANNELS", Message: "There was an error while checking channels: " + err.Error()})
	}
	ReturnResponse(w, res)
}

func HandleGetChannels(w http.ResponseWriter, r *http.Request) {
	log.Info("received a request to get all channels")
	w.Header().Set("Content-Type", "application/json")

	channels, err := GetAll("channels")
	if err != nil {
		res := Response{Type: "Error", Key: "ERROR_GETTING_CHANNELS", Message: "There was an error while getting channels: " + err.Error()}
		json.NewEncoder(w).Encode(res)
		ReturnResponse(w, Response{Type: "Error", Key: "ERROR_GETTING_CHANNELS", Message: "There was an error while getting channels: " + err.Error()})
	}
	json.NewEncoder(w).Encode(channels)
}

func HandleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	log.Info("received a request to delete a channel")

	w.Header().Set("Content-Type", "application/json")
	var data DeleteChannelPayload
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		ReturnResponse(w, Response{Type: "Error", Key: "ERROR_PARSING_DATA", Message: "There was an error parsing json: " + err.Error()})
	}
	channelURL := data.URL
	channelURL = strings.Replace(channelURL, "delChannel", "", -1)
	channel := DownloadTarget{URL: channelURL, Type: "Channel"}

	channel.Delete()

	ReturnResponse(w, Response{Type: "Success", Key: "DELETE_CHANNEL_SUCCESS", Message: "Channel removed"})
}

func HandleUpdateCheckingInterval(w http.ResponseWriter, r *http.Request) {
	log.Info("received a request to update the checking interval")
	w.Header().Set("Content-Type", "application/json")

	var interval CheckingIntervalPayload
	err := json.NewDecoder(r.Body).Decode(&interval)
	if err != nil {
		ReturnResponse(w, Response{Type: "Error", Key: "ERROR_PARSING_DATA", Message: "There was an error parsing json: " + err.Error()})
	}

	res, err := UpdateCheckingInterval(interval.CheckingInterval)
	if err != nil {
		ReturnResponse(w, Response{Type: "Error", Key: "ERROR_UPDATING_CHECKING_INTERVAL", Message: "There was an updating the checking interval: " + err.Error()})
	}
	ReturnResponse(w, res)
}
