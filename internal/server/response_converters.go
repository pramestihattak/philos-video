package server

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	"philos-video/gen/api"
	"philos-video/internal/models"
	videosvc "philos-video/internal/service/video"
)

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

func optInt64(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}

func toResponseUser(u *models.User) api.ResponseUser {
	return api.ResponseUser{
		Id:               u.ID,
		Email:            openapi_types.Email(u.Email),
		Name:             optStr(u.Name),
		Picture:          optStr(u.Picture),
		UploadQuotaBytes: u.UploadQuotaBytes,
		UsedBytes:        u.UsedBytes,
		CreatedAt:        u.CreatedAt,
	}
}

func toResponseComment(c *models.Comment) api.ResponseComment {
	return api.ResponseComment{
		Id:          c.ID,
		VideoId:     c.VideoID,
		UserId:      c.UserID,
		UserName:    c.UserName,
		UserPicture: optStr(c.UserPic),
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
}

func toResponseComments(cs []*models.Comment) []api.ResponseComment {
	out := make([]api.ResponseComment, len(cs))
	for i, c := range cs {
		out[i] = toResponseComment(c)
	}
	return out
}

func toResponseChatMessage(m *models.ChatMessage) api.ResponseChatMessage {
	return api.ResponseChatMessage{
		Id:          m.ID,
		StreamId:    m.StreamID,
		UserId:      m.UserID,
		UserName:    m.UserName,
		UserPicture: optStr(m.UserPic),
		Body:        m.Body,
		CreatedAt:   m.CreatedAt,
	}
}

func toResponseChatMessages(ms []*models.ChatMessage) []api.ResponseChatMessage {
	out := make([]api.ResponseChatMessage, len(ms))
	for i, m := range ms {
		out[i] = toResponseChatMessage(m)
	}
	return out
}

func toResponseVideo(v *models.Video) api.ResponseVideo {
	return api.ResponseVideo{
		Id:              v.ID,
		UserId:          optStr(v.UserID),
		UploaderName:    optStr(v.UploaderName),
		UploaderPicture: optStr(v.UploaderPicture),
		Title:           v.Title,
		Visibility:      api.ResponseVideoVisibility(v.Visibility),
		Status:          api.VideoStatusEnum(v.Status),
		Width:           optInt(v.Width),
		Height:          optInt(v.Height),
		Duration:        optStr(v.Duration),
		Codec:           optStr(v.Codec),
		HlsPath:         optStr(v.HLSPath),
		SizeBytes:       optInt64(v.SizeBytes),
		ThumbnailPath:   optStr(v.ThumbnailPath),
		PlayCount:       v.PlayCount,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
}

func toResponseVideos(vs []*models.Video) []api.ResponseVideo {
	out := make([]api.ResponseVideo, len(vs))
	for i, v := range vs {
		out[i] = toResponseVideo(v)
	}
	return out
}

func toResponseTranscodeJob(j *models.TranscodeJob) *api.ResponseTranscodeJob {
	if j == nil {
		return nil
	}
	return &api.ResponseTranscodeJob{
		Id:        j.ID,
		VideoId:   j.VideoID,
		Status:    api.ResponseTranscodeJobStatus(j.Status),
		Stage:     optStr(j.Stage),
		Progress:  float32(j.Progress),
		Error:     optStr(j.Error),
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}

func toResponseVideoStatus(vs *videosvc.VideoStatus) api.ResponseVideoStatus {
	return api.ResponseVideoStatus{
		Video: toResponseVideo(vs.Video),
		Job:   toResponseTranscodeJob(vs.Job),
	}
}

func toResponseLiveStream(s *models.LiveStream) api.ResponseLiveStream {
	return api.ResponseLiveStream{
		Id:           s.ID,
		UserId:       s.UserID,
		StreamKeyId:  s.StreamKeyID,
		Title:        s.Title,
		Status:       api.ResponseLiveStreamStatus(s.Status),
		RecordVod:    s.RecordVOD,
		SourceWidth:  optInt(s.SourceWidth),
		SourceHeight: optInt(s.SourceHeight),
		SourceCodec:  optStr(s.SourceCodec),
		SourceFps:    optStr(s.SourceFPS),
		HlsPath:      optStr(s.HLSPath),
		VideoId:      optStr(s.VideoID),
		StartedAt:    s.StartedAt,
		EndedAt:      s.EndedAt,
		CreatedAt:    s.CreatedAt,
	}
}

func toResponseLiveStreams(ss []*models.LiveStream) []api.ResponseLiveStream {
	out := make([]api.ResponseLiveStream, len(ss))
	for i, s := range ss {
		out[i] = toResponseLiveStream(s)
	}
	return out
}

func toResponseStreamKey(k *models.StreamKey) api.ResponseStreamKey {
	return api.ResponseStreamKey{
		Id:        k.ID,
		UserLabel: k.UserLabel,
		IsActive:  k.IsActive,
		RecordVod: k.RecordVOD,
		CreatedAt: k.CreatedAt,
	}
}

func toResponseStreamKeys(ks []*models.StreamKey) []api.ResponseStreamKey {
	out := make([]api.ResponseStreamKey, len(ks))
	for i, k := range ks {
		out[i] = toResponseStreamKey(k)
	}
	return out
}
