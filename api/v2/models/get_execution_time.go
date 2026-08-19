package models

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/service"
	"strings"

	"github.com/gin-gonic/gin"
)

type GetSDExecutionTimeInput struct {
	Model        string  `query:"model" description:"HuggingFace model name"`
	Dtype        *string `query:"dtype" description:"Requested dtype"`
	QuantizeBits *int64  `query:"quantize_bits" description:"Requested quantization bits"`
	Variant      *string `query:"variant" description:"SD base-model variant"`
	MinVRAM      *uint64 `query:"min_vram" description:"Minimum GPU VRAM in GB"`
	GPUName      *string `query:"gpu_name" description:"Exact GPU name"`
	GPUVRAM      *uint64 `query:"gpu_vram" description:"Exact GPU VRAM in GB"`
}

type GetLLMExecutionTimeInput struct {
	Model        string  `query:"model" description:"HuggingFace model name"`
	Dtype        *string `query:"dtype" description:"Requested dtype"`
	QuantizeBits *int64  `query:"quantize_bits" description:"Requested quantization bits"`
	MinVRAM      *uint64 `query:"min_vram" description:"Minimum GPU VRAM in GB"`
	GPUName      *string `query:"gpu_name" description:"Exact GPU name"`
	GPUVRAM      *uint64 `query:"gpu_vram" description:"Exact GPU VRAM in GB"`
}

type SDExecutionTimeData struct {
	OverheadSeconds       float64 `json:"overhead_seconds"`
	SecondsPerSDPixelStep float64 `json:"seconds_per_sd_pixel_step"`
}

type LLMExecutionTimeData struct {
	ConstantSeconds       float64 `json:"constant_seconds"`
	SecondsPerInputToken  float64 `json:"seconds_per_input_token"`
	SecondsPerOutputToken float64 `json:"seconds_per_output_token"`
	ModelSwitchSeconds    float64 `json:"model_switch_seconds"`
	SecondsPerImage       float64 `json:"seconds_per_image"`
	SecondsPerMegapixel   float64 `json:"seconds_per_megapixel"`
}

type GetSDExecutionTimeResponse struct {
	response.Response
	Data SDExecutionTimeData `json:"data"`
}

type GetLLMExecutionTimeResponse struct {
	response.Response
	Data LLMExecutionTimeData `json:"data"`
}

type executionTimeRequest struct {
	Model        string
	Dtype        *string
	QuantizeBits *int64
	Variant      *string
	MinVRAM      *uint64
	GPUName      *string
	GPUVRAM      *uint64
}

func GetSDExecutionTime(c *gin.Context, in *GetSDExecutionTimeInput) (*GetSDExecutionTimeResponse, error) {
	query, err := parseExecutionTimeQuery(executionTimeRequest{
		Model: in.Model, Dtype: in.Dtype, QuantizeBits: in.QuantizeBits, Variant: in.Variant,
		MinVRAM: in.MinVRAM, GPUName: in.GPUName, GPUVRAM: in.GPUVRAM,
	})
	if err != nil {
		return nil, err
	}
	coefficients := service.GetSDExecutionTimeCoefficients(query)
	return &GetSDExecutionTimeResponse{
		Data: SDExecutionTimeData{
			OverheadSeconds:       coefficients.OverheadSeconds,
			SecondsPerSDPixelStep: coefficients.SecondsPerSDPixelStep,
		},
	}, nil
}

func GetLLMExecutionTime(c *gin.Context, in *GetLLMExecutionTimeInput) (*GetLLMExecutionTimeResponse, error) {
	query, err := parseExecutionTimeQuery(executionTimeRequest{
		Model: in.Model, Dtype: in.Dtype, QuantizeBits: in.QuantizeBits,
		MinVRAM: in.MinVRAM, GPUName: in.GPUName, GPUVRAM: in.GPUVRAM,
	})
	if err != nil {
		return nil, err
	}
	coefficients := service.GetLLMExecutionTimeCoefficients(query)
	return &GetLLMExecutionTimeResponse{
		Data: LLMExecutionTimeData{
			ConstantSeconds:       coefficients.ConstantSeconds,
			SecondsPerInputToken:  coefficients.SecondsPerInputToken,
			SecondsPerOutputToken: coefficients.SecondsPerOutputToken,
			ModelSwitchSeconds:    coefficients.ModelSwitchSeconds,
			SecondsPerImage:       coefficients.SecondsPerImage,
			SecondsPerMegapixel:   coefficients.SecondsPerMegapixel,
		},
	}, nil
}

func parseExecutionTimeQuery(in executionTimeRequest) (service.TaskExecutionTimeQuery, error) {
	if strings.TrimSpace(in.Model) == "" {
		return service.TaskExecutionTimeQuery{}, response.NewValidationErrorResponse("model", "required")
	}
	if in.QuantizeBits != nil && *in.QuantizeBits < 0 {
		return service.TaskExecutionTimeQuery{}, response.NewValidationErrorResponse("quantize_bits", "must be unsigned")
	}

	hasMinVRAM := in.MinVRAM != nil
	gpuName := ""
	if in.GPUName != nil {
		gpuName = strings.TrimSpace(*in.GPUName)
	}
	hasGPUName := gpuName != ""
	hasGPUVRAM := in.GPUVRAM != nil
	if hasMinVRAM && (hasGPUName || hasGPUVRAM) {
		return service.TaskExecutionTimeQuery{}, response.NewValidationErrorResponse("min_vram", "must not be combined with gpu_name or gpu_vram")
	}
	if hasMinVRAM {
		if *in.MinVRAM == 0 {
			return service.TaskExecutionTimeQuery{}, response.NewValidationErrorResponse("min_vram", "must be a positive integer")
		}
	} else {
		if hasGPUName != hasGPUVRAM {
			field := "gpu_name"
			if hasGPUName {
				field = "gpu_vram"
			}
			return service.TaskExecutionTimeQuery{}, response.NewValidationErrorResponse(field, "gpu_name and gpu_vram must be provided together")
		}
		if !hasGPUName {
			return service.TaskExecutionTimeQuery{}, response.NewValidationErrorResponse("min_vram", "required")
		}
		if *in.GPUVRAM == 0 {
			return service.TaskExecutionTimeQuery{}, response.NewValidationErrorResponse("gpu_vram", "must be a positive integer")
		}
	}

	query := service.TaskExecutionTimeQuery{ModelName: in.Model}
	if in.Dtype != nil && strings.TrimSpace(*in.Dtype) != "" {
		query.RequestedDType = *in.Dtype
	}
	if in.QuantizeBits != nil {
		query.QuantizeBits = uint64(*in.QuantizeBits)
	}
	if in.Variant != nil {
		query.ModelVariant = *in.Variant
	}
	if hasMinVRAM {
		query.MinVRAM = *in.MinVRAM
	} else {
		query.GPUName = gpuName
		query.GPUVRAM = *in.GPUVRAM
	}
	return query, nil
}
