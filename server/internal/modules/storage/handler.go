package storage

import (
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"strconv"
)

// idParam 解析路径参数 id，0 与非法值统一按参数错误处理。
func idParam(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}

// respond 统一出口：有错走 response.Fail，成功走 response.OK。
func respond(c *gin.Context, data any, err error) {
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, data)
}

// handleList 存储资源列表：平台可按 tenant_id/own 过滤，租户仅能查询本租户。
// handleList godoc
// @Summary 分页查询存储资源（租户仅可查询本租户）
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param keyword query string false "文件名"
// @Param tenant_id query int false "平台筛选归属租户，0 为平台"
// @Param own query bool false "只列出当前存储空间"
// @Param status query string false "uploading/ready/deleting"
// @Success 200 {object} response.Body
// @Router /platform/storage/resources [get]
// @Router /tenant/storage/resources [get]
func (s *Service) handleList(c *gin.Context) {
	var in ListQuery
	if err := c.ShouldBindQuery(&in); err != nil {
		response.Fail(c, errcode.Wrap(errcode.ErrInvalidParam, err))
		return
	}
	data, err := s.List(c.Request.Context(), in)
	respond(c, data, err)
}

// handleSummary 返回当前存储空间的套餐、已用容量与上传限制。
// handleSummary godoc
// @Summary 查询当前存储空间套餐、占用和上传限制
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /platform/storage/summary [get]
// @Router /tenant/storage/summary [get]
func (s *Service) handleSummary(c *gin.Context) {
	data, err := s.Summary(c.Request.Context())
	respond(c, data, err)
}

// handleUpload 上传文件到平台 TOS：校验文件名与大小后先预占配额并写台账，
// 上传成功再置为 ready；TOS 未启用时直接拒绝。
// handleUpload godoc
// @Summary 上传文件到平台 TOS 并预占归属空间配额
// @Tags 存储资源
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "文件，默认最大 100 MiB"
// @Success 200 {object} response.Body
// @Router /platform/storage/resources [post]
// @Router /tenant/storage/resources [post]
func (s *Service) handleUpload(c *gin.Context) {
	if s.store == nil {
		response.Fail(c, errcode.Newf(errcode.ErrServiceUnavailable, "平台 TOS 存储未启用"))
		return
	}
	// 比单文件上限多留 1 MiB，超限时由 MaxBytesError 明确归因为请求过大。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.cfg.MaxUploadBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Fail(c, errcode.New(errcode.ErrRequestTooLarge))
		} else {
			response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "请上传有效文件"))
		}
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "缺少 file 文件"))
		return
	}
	body, err := file.Open()
	if err != nil {
		response.Fail(c, errcode.New(errcode.ErrInvalidParam))
		return
	}
	defer body.Close()
	// 先读 512 字节探测 MIME，再回退起点，供后续按原样上传。
	probe := make([]byte, 512)
	n, err := body.Read(probe)
	if err != nil && !errors.Is(err, io.EOF) {
		response.Fail(c, errcode.New(errcode.ErrInvalidParam))
		return
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		response.Fail(c, errcode.New(errcode.ErrInvalidParam))
		return
	}
	data, err := s.Upload(c.Request.Context(), file.Filename, http.DetectContentType(probe[:n]), file.Size, body)
	respond(c, data, err)
}

// handleDelete 删除资源：先标记 deleting 并尝试清理 TOS 对象，失败交由后台任务重试。
// handleDelete godoc
// @Summary 删除资源及 TOS 对象，成功后释放容量，失败自动重试
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "资源 ID"
// @Success 200 {object} response.Body
// @Router /platform/storage/resources/{id} [delete]
// @Router /tenant/storage/resources/{id} [delete]
func (s *Service) handleDelete(c *gin.Context) {
	id, err := idParam(c)
	if err == nil {
		err = s.Delete(c.Request.Context(), id)
	}
	respond(c, nil, err)
}

// handleDownload 生成经归属校验的临时下载地址，不下发任何存储凭证。
// handleDownload godoc
// @Summary 获取经权限校验的临时下载地址
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "资源 ID"
// @Success 200 {object} response.Body
// @Router /platform/storage/resources/{id}/download [get]
// @Router /tenant/storage/resources/{id}/download [get]
func (s *Service) handleDownload(c *gin.Context) {
	id, err := idParam(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	data, err := s.Download(c.Request.Context(), id)
	respond(c, data, err)
}

// handlePlans 读取平台字典 storage.plan 中启用的套餐，仅平台可访问。
// handlePlans godoc
// @Summary 读取平台字典 storage.plan 的启用套餐
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /platform/storage/plans [get]
func (s *Service) handlePlans(c *gin.Context) {
	data, err := s.Plans(c.Request.Context())
	respond(c, data, err)
}

// handleAccounts 平台分页查看各租户存储套餐与占用，缺失账户会按默认套餐补建。
// handleAccounts godoc
// @Summary 查询租户存储套餐和容量占用
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param keyword query string false "租户名称或编码"
// @Success 200 {object} response.Body
// @Router /platform/storage/accounts [get]
func (s *Service) handleAccounts(c *gin.Context) {
	var in ListQuery
	if err := c.ShouldBindQuery(&in); err != nil {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	data, err := s.Accounts(c.Request.Context(), in)
	respond(c, data, err)
}

// handleAllocate 平台为租户分配套餐（assign）或追加容量（expand），并记录配额变更。
// handleAllocate godoc
// @Summary 为租户分配存储套餐或追加容量
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "租户 ID"
// @Param body body AllocationInput true "assign 分配套餐保留扩容，expand 追加字节容量"
// @Success 200 {object} response.Body
// @Router /platform/storage/accounts/{id} [put]
func (s *Service) handleAllocate(c *gin.Context) {
	id, err := idParam(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in AllocationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond(c, nil, errcode.Wrap(errcode.ErrInvalidParam, err))
		return
	}
	data, err := s.Allocate(c.Request.Context(), id, in)
	respond(c, data, err)
}

// handleChanges 分页查询指定租户的存储配额变更记录。
// handleChanges godoc
// @Summary 分页查询租户存储配额变更记录
// @Tags 存储资源
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "租户 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /platform/storage/accounts/{id}/changes [get]
func (s *Service) handleChanges(c *gin.Context) {
	id, err := idParam(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in ListQuery
	if err := c.ShouldBindQuery(&in); err != nil {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	data, err := s.Changes(c.Request.Context(), id, in)
	respond(c, data, err)
}
