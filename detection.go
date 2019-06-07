package main

import (
	speech "cloud.google.com/go/speech/apiv1"
	"context"
	"fmt"
	"github.com/gordonklaus/portaudio"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
	"io"
	"io/ioutil"
	"log"
	"os"
)

type Detection struct {
	Transcript string
	Confidence float32
}

const (
	sampleRate = 256 // 44100 (anticipate 1024 bytw buffer?)
	seconds    = 1
)

// N bytes / sec (assum 16b / sample) [bytes / 4] == sampleRate

func DetectFromFlac(filename string) []Detection {
	ctx := context.Background()
	client, err := speech.NewClient(ctx)
	if err != nil {
		panic(err)
	}
	// read audio file into memory
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	resp, err := client.Recognize(ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:     speechpb.RecognitionConfig_FLAC,
			LanguageCode: "en-us",
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: data},
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n%d results\n", resp, len(resp.Results))
	var detections = make([]Detection, 0)
	for _, result := range resp.Results {
		for _, alt := range result.Alternatives {
			detection := Detection{
				Transcript: alt.Transcript,
				Confidence: alt.Confidence,
			}
			detections = append(detections, detection)
			//fmt.Printf("\"%v\" (confidence=%3f)\n", alt.Transcript, alt.Confidence)
			fmt.Printf("%+v\n", detection)
		}
	}
	return detections
}

// stream from mic
func DetectionStreaming() {
	ctx := context.Background()
	client, err := speech.NewClient(ctx)
	if err != nil {
		panic(err)
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		panic(err)
	}
	// Send the initial configuration message.
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: 16000,
					LanguageCode:    "en-US",
				},
			},
		},
	}); err != nil {
		panic(err)
	}

	err := portaudio.Initialize()
	if err != nil {
		panic(err)
	}
	defer portaudio.Terminate()
	buffer := make([]float32, sampleRate * seconds)
	micStream, err := portaudio.OpenDefaultStream(1, 0, sampleRate,   len(buffer), func(in []float32) {
		for i := range buffer {
			buffer[i] = in[i]
		}
	})
	if err != nil {
		panic(err)
	}
	err = micStream.Start()
	if err != nil {
		panic(err)
	}
	defer micStream.Close()

	go func() {
		buf := make([]byte, 1024)
		for {
			buf = []byte(buffer)
			if err := stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
					AudioContent: buf,
				},
			}); err != nil {
				panic(err)
			}
			if err == io.EOF {
				// Nothing else to pipe, close the stream.
				if err := stream.CloseSend(); err != nil {
					panic(err)
				}
				return
			}
			if err != nil {
				panic(err)
			}
		}
	}()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if err := resp.Error; err != nil {
			// Workaround while the API doesn't give a more informative error.
			if err.Code == 3 || err.Code == 11 {
				log.Print("WARNING: Speech recognition request exceeded limit of 60 seconds.")
			}
			log.Fatalf("Could not recognize: %v", err)
		}
		for _, result := range resp.Results {
			for _, alt := range result.Alternatives {
				detection := Detection{
					Transcript: alt.Transcript,
					Confidence: alt.Confidence,
				}
				fmt.Printf("%+v\n", detection)
			}
		}
	}
}

func DetectionMain() {
	DetectionStreaming()
}
