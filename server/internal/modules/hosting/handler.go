package hosting

import (
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"

	"github.com/gin-gonic/gin"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/router"
)

// pathID 解析并校验路径参数 :id（必须为正整数）。
func pathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}

// optionalID 解析可选路径参数：为空表示新增（返回 0），否则按 :id 校验。
func optionalID(c *gin.Context) (uint64, error) {
	if c.Param("id") == "" {
		return 0, nil
	}
	return pathID(c)
}

// respond 统一把业务错误转为失败响应，否则按成功返回。
func respond(c *gin.Context, data any, err error) {
	if err != nil {
		response.Fail(c, err)
	} else {
		response.OK(c, data)
	}
}

// bind 以 2 MiB 上限读取并解析 JSON 请求体，失败时直接写回参数错误。
func bind(c *gin.Context, in any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	if err := c.ShouldBindJSON(in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法"))
		return false
	}
	return true
}

// handleServers lists platform SSH servers.
// @Summary 服务器分页列表
// @Tags 平台-服务器管理
// @Security BearerAuth
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "分页大小"
// @Param keyword query string false "名称/主机"
// @Success 200 {object} response.Body "data 为 response.Page[model.ManagedServer]"
// @Router /platform/servers [get]
func (s *Service) handleServers(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	data, err := s.ListServers(c.Request.Context(), q)
	respond(c, data, err)
}

// handleSaveServer creates or updates encrypted platform SSH credentials.
// @Summary 新增/编辑服务器（空凭证保留原值）
// @Tags 平台-服务器管理
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body ServerInput true "服务器"
// @Success 200 {object} response.Body
// @Router /platform/servers [post]
func (s *Service) handleSaveServer(c *gin.Context) {
	id, err := optionalID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in ServerInput
	if !bind(c, &in) {
		return
	}
	data, err := s.SaveServer(c.Request.Context(), id, in)
	respond(c, data, err)
}

// @Summary 编辑服务器（空凭证保留原值）
// @Tags 平台-服务器管理
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "服务器 ID"
// @Param body body ServerInput true "服务器"
// @Success 200 {object} response.Body
// @Router /platform/servers/{id} [put]
func (s *Service) handleUpdateServer(c *gin.Context) { s.handleSaveServer(c) }

// @Summary 删除未关联网站的服务器
// @Tags 平台-服务器管理
// @Security BearerAuth
// @Param id path int true "服务器 ID"
// @Success 200 {object} response.Body
// @Router /platform/servers/{id} [delete]
func (s *Service) handleDeleteServer(c *gin.Context) {
	id, err := pathID(c)
	if err == nil {
		err = s.DeleteServer(c.Request.Context(), id)
	}
	respond(c, nil, err)
}

// @Summary 测试 SSH 连接和远程账号权限
// @Tags 平台-服务器管理
// @Security BearerAuth
// @Param id path int true "服务器 ID"
// @Success 200 {object} response.Body "data 为 CommandResult"
// @Router /platform/servers/{id}/test [post]
func (s *Service) handleTestServer(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	data, err := s.Execute(c.Request.Context(), id, "id; uname -s; command -v docker || true")
	respond(c, data, err)
}

type CommandInput struct {
	Command string `json:"command" binding:"required,max=16384"`
}

// @Summary 执行平台 SSH 命令（使用配置账号的系统权限）
// @Tags 平台-服务器管理
// @Security BearerAuth
// @Accept json
// @Param id path int true "服务器 ID"
// @Param body body CommandInput true "命令"
// @Success 200 {object} response.Body "data 为 CommandResult"
// @Router /platform/servers/{id}/exec [post]
func (s *Service) handleExec(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in CommandInput
	if !bind(c, &in) {
		return
	}
	data, err := s.Execute(c.Request.Context(), id, in.Command)
	respond(c, data, err)
}

// @Summary 网站分页列表（租户仅查询本租户网站）
// @Tags 网站管理
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "分页大小"
// @Param keyword query string false "名称/地址"
// @Success 200 {object} response.Body "data 为 response.Page[model.Website]"
// @Router /platform/websites [get]
// @Router /tenant/websites [get]
func (s *Service) handleWebsites(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	data, err := s.ListWebsites(c.Request.Context(), q)
	respond(c, data, err)
}

// @Summary 分配/编辑网站（归属和服务器不可更改）
// @Tags 平台-网站管理
// @Security BearerAuth
// @Accept json
// @Param body body WebsiteInput true "网站"
// @Success 200 {object} response.Body
// @Router /platform/websites [post]
func (s *Service) handleSaveWebsite(c *gin.Context) {
	id, err := optionalID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in WebsiteInput
	if !bind(c, &in) {
		return
	}
	data, err := s.SaveWebsite(c.Request.Context(), id, in)
	respond(c, data, err)
}

// @Summary 编辑网站（归属和服务器不可更改）
// @Tags 平台-网站管理
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "网站 ID"
// @Param body body WebsiteInput true "网站"
// @Success 200 {object} response.Body
// @Router /platform/websites/{id} [put]
func (s *Service) handleUpdateWebsite(c *gin.Context) { s.handleSaveWebsite(c) }

// @Summary 初始化独立账号及 chroot 网站目录
// @Tags 平台-网站管理
// @Security BearerAuth
// @Param id path int true "网站 ID"
// @Success 200 {object} response.Body
// @Router /platform/websites/{id}/provision [post]
func (s *Service) handleProvision(c *gin.Context) {
	id, err := pathID(c)
	if err == nil {
		err = s.Provision(c.Request.Context(), id)
	}
	respond(c, nil, err)
}

// @Summary 部署独立 Directus、数据库及持久化卷
// @Tags 平台-网站管理
// @Security BearerAuth
// @Accept json
// @Param id path int true "网站 ID"
// @Param body body DeployInput true "部署参数"
// @Success 200 {object} response.Body
// @Router /platform/websites/{id}/cms/deploy [post]
func (s *Service) handleDeployCMS(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in DeployInput
	if !bind(c, &in) {
		return
	}
	respond(c, nil, s.DeployCMS(c.Request.Context(), id, in))
}

// registerFiles 在平台/租户两组路由上挂载同一套网站文件接口，
// 复用 service 层的租户作用域，scope 仅决定权限点前缀。
func (s *Service) registerFiles(g *gin.RouterGroup, r *router.Router, scope string) {
	g.GET("/:id/files", r.Require(scope+".website.files"), s.handleFiles)
	g.GET("/:id/files/content", r.Require(scope+".website.files"), s.handleRead)
	g.GET("/:id/files/download", r.Require(scope+".website.files"), s.handleDownload)
	g.PUT("/:id/files/content", r.Require(scope+".website.write"), s.handleWrite)
	g.POST("/:id/files/upload", r.Require(scope+".website.write"), s.handleUpload)
	g.POST("/:id/files/actions", r.Require(scope+".website.write"), s.handleFileAction)
}

// @Summary 浏览网站目录
// @Tags 网站文件
// @Security BearerAuth
// @Param id path int true "网站 ID"
// @Param path query string false "相对路径"
// @Success 200 {object} response.Body "data 为 []FileItem"
// @Router /platform/websites/{id}/files [get]
// @Router /tenant/websites/{id}/files [get]
func (s *Service) handleFiles(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	data, err := s.ListFiles(c.Request.Context(), id, c.Query("path"))
	respond(c, data, err)
}

// @Summary 读取 UTF-8 文本（最大 1 MiB）
// @Tags 网站文件
// @Security BearerAuth
// @Param id path int true "网站 ID"
// @Param path query string true "文件路径"
// @Success 200 {object} response.Body "data 为 content 字符串"
// @Router /platform/websites/{id}/files/content [get]
// @Router /tenant/websites/{id}/files/content [get]
func (s *Service) handleRead(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	data, err := s.ReadFile(c.Request.Context(), id, c.Query("path"), true)
	respond(c, gin.H{"content": string(data)}, err)
}

// @Summary 下载网站文件（最大 10 MiB）
// @Tags 网站文件
// @Security BearerAuth
// @Param id path int true "网站 ID"
// @Param path query string true "文件路径"
// @Success 200 {file} binary
// @Router /platform/websites/{id}/files/download [get]
// @Router /tenant/websites/{id}/files/download [get]
func (s *Service) handleDownload(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	data, err := s.ReadFile(c.Request.Context(), id, c.Query("path"), false)
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(c.Query("path"))}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

type TextInput struct {
	Path    string `json:"path" binding:"required,max=1024"`
	Content string `json:"content"`
}

// @Summary 保存文本文件（最大 1 MiB）
// @Tags 网站文件
// @Security BearerAuth
// @Accept json
// @Param id path int true "网站 ID"
// @Param body body TextInput true "文本"
// @Success 200 {object} response.Body
// @Router /platform/websites/{id}/files/content [put]
// @Router /tenant/websites/{id}/files/content [put]
func (s *Service) handleWrite(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in TextInput
	if !bind(c, &in) {
		return
	}
	if len(in.Content) > maxTextBytes {
		respond(c, nil, errcode.New(errcode.ErrRequestTooLarge))
		return
	}
	respond(c, nil, s.WriteFile(c.Request.Context(), id, in.Path, []byte(in.Content)))
}

// @Summary 上传网站文件（最大 10 MiB）
// @Tags 网站文件
// @Security BearerAuth
// @Accept multipart/form-data
// @Param id path int true "网站 ID"
// @Param path formData string true "目标文件路径"
// @Param file formData file true "文件"
// @Success 200 {object} response.Body
// @Router /platform/websites/{id}/files/upload [post]
// @Router /tenant/websites/{id}/files/upload [post]
func (s *Service) handleUpload(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		respond(c, nil, errcode.New(errcode.ErrRequestTooLarge))
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	respond(c, nil, s.WriteFile(c.Request.Context(), id, c.PostForm("path"), data))
}

type FileAction struct {
	Operation   string `json:"operation" binding:"required,oneof=mkdir rename delete"`
	Path        string `json:"path" binding:"required,max=1024"`
	Destination string `json:"destination" binding:"max=1024"`
}

// @Summary 新建目录、重命名或删除文件/空目录
// @Tags 网站文件
// @Security BearerAuth
// @Accept json
// @Param id path int true "网站 ID"
// @Param body body FileAction true "操作"
// @Success 200 {object} response.Body
// @Router /platform/websites/{id}/files/actions [post]
// @Router /tenant/websites/{id}/files/actions [post]
func (s *Service) handleFileAction(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in FileAction
	if !bind(c, &in) {
		return
	}
	if in.Operation == "rename" && in.Destination == "" {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	respond(c, nil, s.MutateFile(c.Request.Context(), id, in.Operation, in.Path, in.Destination))
}
