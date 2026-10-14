package service_test

import (
	"crynux_relay/models"
	"crynux_relay/service"
	"database/sql"
	"math/big"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestTaskQueue(t *testing.T) {
	tasks := make([]*models.InferenceTask, 0)
	tasks = append(tasks, &models.InferenceTask{
		Model: gorm.Model{
			ID: 1,
		},
		TaskArgs:         `{"base_model":{"name":"crynux-ai/sdxl-turbo", "variant": "fp16"},"prompt":"Self-portrait oil painting,a beautiful cyborg with golden hair,8k","negative_prompt":"","scheduler":{"method":"EulerAncestralDiscreteScheduler","args":{"timestep_spacing":"trailing"}},"task_config":{"num_images_per_prompt":1,"seed":42,"steps":1,"cfg":0}}`,
		TaskIDCommitment: "0x342ca5ec79994b43f62598cbf5affad9cc0906a7ef21169cae25b61c370b43bf",
		Creator:          "0x72E420eCAF65263Dd3246601Adf15DdDDfB91774",
		SamplingSeed:     "0x7712a81425e865bddce9da146e0f697688b24edf162c4c6a70f80628583ddf8a",
		Nonce:            "0x97296ec7888afb041cde5201179da43c36b8f9b652154ec772ec2a928df821bb",
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeSD,
		TaskVersion:      "2.5.0",
		MinVRAM:          4,
		TaskFee:          models.BigInt{Int: *big.NewInt(4100000000)},
		TaskSize:         1,
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	})
	tasks = append(tasks, &models.InferenceTask{
		Model: gorm.Model{
			ID: 2,
		},
		TaskArgs:         `{"base_model":{"name":"crynux-ai/stable-diffusion-v1-5", "variant": "fp16"},"prompt":"best quality, ultra high res, photorealistic++++, 1girl, off-shoulder sweater, smiling, faded ash gray messy bun hair+, border light, depth of field, looking at viewer, closeup","task_config":{"num_images_per_prompt":1,"seed":42,"steps":25,"cfg":0,"safety_checker":false}}`,
		TaskIDCommitment: "0xf3bab6937aad9470cc0a6ec0134c11f5858f483d9c4bda9390f8bf5b0c1e5e63",
		Creator:          "0x72E420eCAF65263Dd3246601Adf15DdDDfB91774",
		SamplingSeed:     "0x92d56cd5aa169b0b4babdeebb112ef1abd73303325ebc3d5b29450f1aa521a30",
		Nonce:            "0xbf4c785ca5d46f707639756996f81b7f2dcf0a4eda8b426a762c71e82bc4af69",
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeSD,
		TaskVersion:      "2.5.0",
		MinVRAM:          4,
		TaskFee:          models.BigInt{Int: *big.NewInt(4100000000)},
		TaskSize:         1,
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	})
	tasks = append(tasks, &models.InferenceTask{
		Model: gorm.Model{
			ID: 3,
		},
		TaskArgs:         `{"model":"Qwen/Qwen2.5-7B","messages":[{"role":"user","content":"I want to create an AI agent. Any suggestions?"}],"tools":null,"generation_config":{"max_new_tokens":250,"do_sample":true,"temperature":0.8,"repetition_penalty":1.1},"seed":42,"dtype":"bfloat16","quantize_bits":4}`,
		TaskIDCommitment: "0xcd6d403f21c5af106f39c2e1ad921a3e9caf97a1017c3802798e47ad261742b3",
		Creator:          "0x72E420eCAF65263Dd3246601Adf15DdDDfB91774",
		SamplingSeed:     "0x574cfc0049946ba0d268d6659d00af8641e1fa62032f6f9004c71d53276b960f",
		Nonce:            "0xb9156b44da096be016f90da4434891bf81009e51b5ee296dfef65bf363340d80",
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeLLM,
		TaskVersion:      "2.5.0",
		MinVRAM:          4,
		TaskFee:          models.BigInt{Int: *big.NewInt(4100000000)},
		TaskSize:         1,
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	})
	tasks = append(tasks, &models.InferenceTask{
		Model: gorm.Model{
			ID: 4,
		},
		TaskArgs:         `{"model":"Qwen/Qwen2.5-7B","messages":[{"role":"user","content":"I want to create an AI agent. Any suggestions?"}],"tools":null,"generation_config":{"max_new_tokens":250,"do_sample":true,"temperature":0.8,"repetition_penalty":1.1},"seed":42,"dtype":"bfloat16","quantize_bits":4}`,
		TaskIDCommitment: "0xe46a7df276680fab5be8b245469e7c5d96e112d86812953d983f9beac5100754",
		Creator:          "0x72E420eCAF65263Dd3246601Adf15DdDDfB91774",
		SamplingSeed:     "0x0410e745020ade9f62c35ecae98fdca004f3e7ea34e0547925d2dc9ecd683f84",
		Nonce:            "0x9c8310739f5c54a8b1987d8c8c87f33a2b77d9852e9c8f59e7a399fce6cbc70a",
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeLLM,
		TaskVersion:      "2.5.0",
		MinVRAM:          4,
		TaskFee:          models.BigInt{Int: *big.NewInt(4100000000)},
		TaskSize:         1,
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	})

	queue := service.NewTaskQueue()
	queue.Push(tasks...)
	orders := []uint{3,4,1,2}
	for i := 0; i < 4; i++ {
		task, _ := queue.Pop()
		if task.ID != orders[i] {
			t.Fatal("Wrong task order")
		}
	}
}
