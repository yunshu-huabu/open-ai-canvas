# Vinted Seedance Video 接口字段

## 协议身份

- 插件 ID：`vinted-seedance-video`。
- Provider ID：`vinted-seedance-video`。
- 能力：`video`。
- 默认 Base URL：`https://vinted.cam`。
- 鉴权驱动：`bearer`。
- 创建：`POST /v1/videos`。
- 查询：`GET /v1/videos/{{taskId}}`。

## 配置字段

| 字段 | 类型 | 必填 | 含义 |
| --- | --- | --- | --- |
| `apiKey` | secret | 是 | API Key |

## 统一字段映射

| 统一字段 | 类型 | 必填 | 上游映射 | 说明 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | `model` | 视频模型 ID。 |
| `prompt` | string | 是 | `prompt/content/input` | 视频提示词。 |
| `images` | media[] | 否 | `first/last/reference image` | 显式 role 图片输入。 |
| `videos` | media[] | 否 | `reference video` | 参考视频。 |
| `audios` | media[] | 否 | `reference audio/voice` | 参考音频或音色。 |
| `duration` | integer | 否 | `duration/seconds` | 时长秒数。 |
| `aspectRatio` | string | 否 | `ratio/aspect_ratio/size` | 画幅比例或尺寸。 |
| `resolution` | string | 否 | `resolution` | 分辨率档位。 |
| `providerOptions` | object | 否 | `provider-specific fields` | 插件命名空间内的厂商扩展字段。 |

## 上游请求模板逐字段清单

下表由插件请求模板生成，覆盖 body、query、headers 和 multipart 文件声明中的每个字段。

| 上游位置 | 值或转换表达式 |
| --- | --- |
| `create.method` | `"POST"` |
| `create.path` | `"/v1/videos"` |
| `create.contentType` | `"application/json"` |
| `create.body.model` | `{"$ref":"request.model"}` |
| `create.body.prompt` | `{"$ref":"request.prompt"}` |
| `create.body.duration` | `{"$if":{"condition":{"$eq":[{"$ref":"request.duration"},0]},"then":{"$if":{"condition":{"$eq":[{"$ref":"request.model"},"seedance2.5"]},"then":30,"else":5}},"else":{"$ref":"request.duration"}}}` |
| `create.body.ratio` | `{"$omitEmpty":{"$ref":"request.aspectRatio"}}` |
| `create.body.resolution` | `{"$coalesce":[{"$ref":"request.resolution"},"720p"]}` |
| `create.body.camera_movement` | `{"$omitEmpty":{"$ref":"request.providerOptions.vinted-seedance-video.camera_movement"}}` |
| `create.body.native_face` | `{"$ref":"request.providerOptions.vinted-seedance-video.native_face"}` |
| `create.body.image_ids` | `{"$omitEmpty":{"$concatArrays":[{"$ref":"request.providerOptions.vinted-seedance-video.image_ids"},{"$ref":"request.extra.preparation.images"}]}}` |
| `create.body.video_b64s` | `{"$omitEmpty":{"$map":{"from":{"$sortByOrder":{"$ref":"request.videos"}},"as":"media","in":{"$ref":"media.dataUrl"}}}}` |
| `create.body.audio_b64s` | `{"$omitEmpty":{"$map":{"from":{"$sortByOrder":{"$ref":"request.audios"}},"as":"media","in":{"$ref":"media.dataUrl"}}}}` |
| `create.headers.Idempotency-Key` | `{"$ref":"request.extra.idempotencyKey"}` |
| `create.headers.User-Agent` | `"vinted-video-client/1.0"` |
| `poll.method` | `"GET"` |
| `poll.path` | `"/v1/videos/{{taskId}}"` |
| `poll.contentType` | `"application/json"` |
| `poll.headers.User-Agent` | `"vinted-video-client/1.0"` |
| `result.method` | `"GET"` |
| `result.path` | `"/v1/videos/{{taskId}}/signed_url"` |
| `result.contentType` | `"application/json"` |
| `result.query.download` | `"1"` |
| `result.headers.User-Agent` | `"vinted-video-client/1.0"` |

## Provider 扩展键

- `providerOptions.vinted-seedance-video.camera_movement`
- `providerOptions.vinted-seedance-video.image_ids`
- `providerOptions.vinted-seedance-video.native_face`

动态模型或工作流允许使用文档声明的完整 `parameters/input/extra_body` 对象；该对象是协议本身的开放 schema，不会被宿主裁剪。

## 响应映射逐字段清单

| 映射位置 | 上游路径或转换表达式 |
| --- | --- |
| `response.taskId` | `{"$coalesce":[{"$ref":"response.id"},{"$ref":"taskId"}]}` |
| `response.status` | `{"$if":{"condition":{"$eq":[{"$ref":"response.quality"},"original"]},"then":"succeeded","else":{"$coalesce":[{"$ref":"response.status"},"failed"]}}}` |
| `response.videos` | `{"$if":{"condition":{"$eq":[{"$ref":"response.quality"},"original"]},"then":{"$ref":"response.url"},"else":null}}` |
| `response.message` | `{"$coalesce":[{"$ref":"response.error.message"},{"$ref":"response.error"},{"$ref":"response.message"},"原片签名响应未确认 quality=original 或未提供有效地址，请恢复原任务"]}` |
| `response.errorPaths[0]` | `"error"` |
| `response.resultEphemeral` | `true` |

## 响应与错误

插件把上游 task/status/text/media/usage 映射为统一结果。临时媒体 URL 标记为 ephemeral，由宿主立即下载持久化。HTTP 错误、业务 code 和 error object 保持失败语义，不包装成成功。

## 兼容边界

仅对应 https://vinted.cam/developers 主模块的四个 Seedance 模型，不包含其他模型。安装包使用 declarative-lifecycle 运行时，必须同时更新本轮后端；旧宿主会拒绝安装，避免漏传参考素材。渠道地址填 https://vinted.cam（不含 /v1），API Key 填原 sk- Key。2.5 为 16–30 秒整数（默认 30），2.0/Fast 为 5/10/15 秒（默认 5），Mini 为 5/10 秒（默认 5）；分辨率仅 720p。

每次创建前读取 /v1/catalog、带 Key 的 /v1/pricing 和 /v1/balance，校验可售时长、本人价格和余额；报价包含适用的原生人脸附加费，报价与余额写入任务进度，最终以账单为准。页面的模型下拉列表仍由渠道模型配置管理，不自动新增模型。

宿主读取归属校验后的本地资源或公网直链，校验实际格式与字节大小，图片逐张以 image_b64 上传 /v1/files，音频和视频以 audio_b64s/video_b64s 原始文件提交。上传图片单张 50 MiB；音视频单条 50 MiB、每类合计 60 MiB，JSON 总体积仍受上游限制。每次新任务重新上传图片，不持久缓存一小时有效的上传 ID。可通过 providerOptions.vinted-seedance-video.image_ids 追加已有有效图片 ID（须合法且不重复）；已有 ID 排在自动上传图片之前，过期且没有原文件的 ID 必须由用户重新提供，不能静默删除素材。2.5 图片/视频/音频上限分别为 30/10/10，其余为 9/3/3。三类素材各自按 order 排序，对应 @图片N、@视频N、@音频N。

视频设置提供运镜和原生人脸选项，也可通过渠道模型 defaultOptions.camera_movement（auto/fixed）和 native_face（布尔值）配置。超过 15 秒时原生人脸自动无效；15 秒内开启须有参考图或视频。省略时保留上游默认，可能自动开启并加收积分。

创建异常、空响应或任务 ID 缺失时最多三次使用同一 Idempotency-Key 与相同正文重试；429 支持响应头和 JSON retry_after。收到任务 ID 后只查询原任务，每 5 秒轮询。进程重启且创建响应未保存为任务 ID 时仍遵守宿主的保守暂停规则，不自动创建新订单，需核对原任务。

原片使用 /signed_url?download=1，验证 quality=original、文件 X-Video-Quality=original、Content-Type 和 MP4 文件头。签名地址无需鉴权，不附加渠道 Key；过期则重新获取同任务签名。下载中断时，在同次执行中使用 Range/If-Range 和 ETag 续传，严格检查版本与字节范围；进程重启后重新下载，不持久保存半文件。文件成功保存前不报告完成。未使用真实 Key 联调或创建计费任务。

<!-- YINGCE_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v2",
  "runtime": {
    "backend": "declarative-lifecycle"
  },
  "id": "vinted-seedance-video",
  "name": "Vinted Seedance Video",
  "version": "2.1.0",
  "author": "Vinted / 影策 / 影策",
  "description": "Vinted Seedance Video 独立请求协议插件。",
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>",
  "permissions": [
    "generation.run",
    "media.read"
  ],
  "configuration": {
    "fields": [
      {
        "name": "apiKey",
        "type": "secret",
        "label": "API Key",
        "required": true
      }
    ]
  },
  "contributes": {
    "providers": [
      {
        "id": "vinted-seedance-video",
        "label": "Vinted Seedance Video",
        "capabilities": [
          "video"
        ],
        "scopes": [
          "admin.system-channel",
          "user.custom-channel",
          "canvas",
          "creation",
          "agent"
        ],
        "baseUrl": "https://vinted.cam",
        "requiresPublicMediaUrls": false,
        "auth": {
          "type": "bearer",
          "field": "apiKey"
        },
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "required": true,
            "mapping": "model",
            "description": "视频模型 ID。"
          },
          {
            "name": "prompt",
            "type": "string",
            "required": true,
            "mapping": "prompt/content/input",
            "description": "视频提示词。"
          },
          {
            "name": "images",
            "type": "media[]",
            "required": false,
            "mapping": "first/last/reference image",
            "description": "显式 role 图片输入。"
          },
          {
            "name": "videos",
            "type": "media[]",
            "required": false,
            "mapping": "reference video",
            "description": "参考视频。"
          },
          {
            "name": "audios",
            "type": "media[]",
            "required": false,
            "mapping": "reference audio/voice",
            "description": "参考音频或音色。"
          },
          {
            "name": "duration",
            "type": "integer",
            "required": false,
            "mapping": "duration/seconds",
            "description": "时长秒数。"
          },
          {
            "name": "aspectRatio",
            "type": "string",
            "required": false,
            "mapping": "ratio/aspect_ratio/size",
            "description": "画幅比例或尺寸。"
          },
          {
            "name": "resolution",
            "type": "string",
            "required": false,
            "mapping": "resolution",
            "description": "分辨率档位。"
          },
          {
            "name": "providerOptions",
            "type": "object",
            "required": false,
            "mapping": "provider-specific fields",
            "description": "插件命名空间内的厂商扩展字段。"
          }
        ],
        "validations": [
          {
            "assert": {
              "$gt": [
                {
                  "$len": {
                    "$trim": {
                      "$ref": "request.extra.idempotencyKey"
                    }
                  }
                },
                0
              ]
            },
            "message": "Vinted 创建视频必须由宿主提供 Idempotency-Key，请升级后端"
          },
          {
            "assert": {
              "$in": [
                {
                  "$ref": "request.model"
                },
                [
                  "seedance2.5",
                  "seedance2.0",
                  "seedance2.0fast",
                  "seedance2.0mini"
                ]
              ]
            },
            "message": "Vinted 主模块仅支持四个 Seedance 模型，不支持 niulai/C4/C6"
          },
          {
            "assert": {
              "$and": [
                {
                  "$gt": [
                    {
                      "$len": {
                        "$trim": {
                          "$ref": "request.prompt"
                        }
                      }
                    },
                    0
                  ]
                },
                {
                  "$lte": [
                    {
                      "$len": {
                        "$ref": "request.prompt"
                      }
                    },
                    15000
                  ]
                }
              ]
            },
            "message": "Vinted 提示词须为 1–15000 字符"
          },
          {
            "assert": {
              "$if": {
                "condition": {
                  "$eq": [
                    {
                      "$ref": "request.model"
                    },
                    "seedance2.5"
                  ]
                },
                "then": {
                  "$in": [
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.duration"
                            },
                            0
                          ]
                        },
                        "then": {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "seedance2.5"
                              ]
                            },
                            "then": 30,
                            "else": 5
                          }
                        },
                        "else": {
                          "$ref": "request.duration"
                        }
                      }
                    },
                    [
                      16,
                      17,
                      18,
                      19,
                      20,
                      21,
                      22,
                      23,
                      24,
                      25,
                      26,
                      27,
                      28,
                      29,
                      30
                    ]
                  ]
                },
                "else": {
                  "$in": [
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.duration"
                            },
                            0
                          ]
                        },
                        "then": {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "seedance2.5"
                              ]
                            },
                            "then": 30,
                            "else": 5
                          }
                        },
                        "else": {
                          "$ref": "request.duration"
                        }
                      }
                    },
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.model"
                            },
                            "seedance2.0mini"
                          ]
                        },
                        "then": [
                          5,
                          10
                        ],
                        "else": [
                          5,
                          10,
                          15
                        ]
                      }
                    }
                  ]
                }
              }
            },
            "message": "Vinted 时长不合法：2.5 为 16–30 秒整数，2.0/Fast 为 5/10/15，Mini 为 5/10"
          },
          {
            "assert": {
              "$in": [
                {
                  "$ref": "request.aspectRatio"
                },
                [
                  "",
                  "1:1",
                  "3:4",
                  "4:3",
                  "9:16",
                  "16:9",
                  "21:9"
                ]
              ]
            },
            "message": "Vinted 不支持此画幅比例"
          },
          {
            "assert": {
              "$in": [
                {
                  "$coalesce": [
                    {
                      "$ref": "request.resolution"
                    },
                    "720p"
                  ]
                },
                [
                  "720p"
                ]
              ]
            },
            "message": "Vinted 主模块仅支持 720p"
          },
          {
            "assert": {
              "$or": [
                {
                  "$eq": [
                    {
                      "$len": {
                        "$ref": "request.providerOptions.vinted-seedance-video.camera_movement"
                      }
                    },
                    0
                  ]
                },
                {
                  "$in": [
                    {
                      "$ref": "request.providerOptions.vinted-seedance-video.camera_movement"
                    },
                    [
                      "auto",
                      "fixed"
                    ]
                  ]
                }
              ]
            },
            "message": "Vinted 运镜仅支持 auto 或 fixed"
          },
          {
            "assert": {
              "$in": [
                {
                  "$coalesce": [
                    {
                      "$ref": "request.providerOptions.vinted-seedance-video.native_face"
                    },
                    false
                  ]
                },
                [
                  true,
                  false
                ]
              ]
            },
            "message": "Vinted native_face 必须是布尔值"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$add": [
                    {
                      "$len": {
                        "$ref": "request.images"
                      }
                    },
                    {
                      "$len": {
                        "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                      }
                    }
                  ]
                },
                {
                  "$if": {
                    "condition": {
                      "$eq": [
                        {
                          "$ref": "request.model"
                        },
                        "seedance2.5"
                      ]
                    },
                    "then": 30,
                    "else": 9
                  }
                }
              ]
            },
            "message": "Vinted 参考图数量超限（含 image_ids）"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$ref": "request.videos"
                  }
                },
                {
                  "$if": {
                    "condition": {
                      "$eq": [
                        {
                          "$ref": "request.model"
                        },
                        "seedance2.5"
                      ]
                    },
                    "then": 10,
                    "else": 3
                  }
                }
              ]
            },
            "message": "Vinted 参考音视频数量超限"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$ref": "request.audios"
                  }
                },
                {
                  "$if": {
                    "condition": {
                      "$eq": [
                        {
                          "$ref": "request.model"
                        },
                        "seedance2.5"
                      ]
                    },
                    "then": 10,
                    "else": 3
                  }
                }
              ]
            },
            "message": "Vinted 参考音视频数量超限"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                  }
                },
                {
                  "$len": {
                    "$unique": {
                      "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                    }
                  }
                }
              ]
            },
            "message": "Vinted 图片 ID 不能重复"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                      },
                      "as": "id",
                      "where": {
                        "$not": {
                          "$matches": [
                            {
                              "$ref": "id"
                            },
                            "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"
                          ]
                        }
                      }
                    }
                  }
                },
                0
              ]
            },
            "message": "Vinted 图片 ID 格式不合法"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$ref": "request.providerOptions.vinted-seedance-video.native_face"
                  }
                },
                {
                  "$gt": [
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.duration"
                            },
                            0
                          ]
                        },
                        "then": {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "seedance2.5"
                              ]
                            },
                            "then": 30,
                            "else": 5
                          }
                        },
                        "else": {
                          "$ref": "request.duration"
                        }
                      }
                    },
                    15
                  ]
                },
                {
                  "$gt": [
                    {
                      "$add": [
                        {
                          "$add": [
                            {
                              "$len": {
                                "$ref": "request.images"
                              }
                            },
                            {
                              "$len": {
                                "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                              }
                            }
                          ]
                        },
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        }
                      ]
                    },
                    0
                  ]
                }
              ]
            },
            "message": "15 秒以内开启原生人脸须提供参考图片或视频"
          }
        ],
        "lifecycle": {
          "pollIntervalMs": 5000,
          "createAttempts": 3,
          "originalMp4": true,
          "requestBytes": 100663296,
          "mediaLimits": {
            "images": {
              "mimeTypes": [
                "image/png",
                "image/jpeg",
                "image/webp"
              ],
              "fileBytes": 52428800,
              "totalBytes": 1572864000
            },
            "audios": {
              "mimeTypes": [
                "audio/mpeg",
                "audio/mp4",
                "audio/x-m4a",
                "audio/wav",
                "audio/ogg",
                "audio/flac"
              ],
              "fileBytes": 52428800,
              "totalBytes": 62914560
            },
            "videos": {
              "mimeTypes": [
                "video/mp4",
                "video/quicktime",
                "video/webm"
              ],
              "fileBytes": 52428800,
              "totalBytes": 62914560
            }
          },
          "prepare": [
            {
              "id": "catalog",
              "operation": {
                "method": "GET",
                "path": "/v1/catalog"
              },
              "value": true,
              "validations": [
                {
                  "assert": {
                    "$gt": [
                      {
                        "$len": {
                          "$filter": {
                            "from": {
                              "$ref": "response.models"
                            },
                            "as": "model",
                            "where": {
                              "$and": [
                                {
                                  "$eq": [
                                    {
                                      "$ref": "model.id"
                                    },
                                    {
                                      "$ref": "request.model"
                                    }
                                  ]
                                },
                                {
                                  "$in": [
                                    {
                                      "$if": {
                                        "condition": {
                                          "$eq": [
                                            {
                                              "$ref": "request.duration"
                                            },
                                            0
                                          ]
                                        },
                                        "then": {
                                          "$if": {
                                            "condition": {
                                              "$eq": [
                                                {
                                                  "$ref": "request.model"
                                                },
                                                "seedance2.5"
                                              ]
                                            },
                                            "then": 30,
                                            "else": 5
                                          }
                                        },
                                        "else": {
                                          "$ref": "request.duration"
                                        }
                                      }
                                    },
                                    {
                                      "$ref": "model.durations"
                                    }
                                  ]
                                }
                              ]
                            }
                          }
                        }
                      },
                      0
                    ]
                  },
                  "message": "当前模型或时长已不在上游目录中"
                }
              ]
            },
            {
              "id": "pricing",
              "operation": {
                "method": "GET",
                "path": "/v1/pricing"
              },
              "value": {
                "credits": {
                  "$add": [
                    {
                      "$first": {
                        "$map": {
                          "from": {
                            "$filter": {
                              "from": {
                                "$ref": "response.items"
                              },
                              "as": "price",
                              "where": {
                                "$and": [
                                  {
                                    "$eq": [
                                      {
                                        "$ref": "price.model"
                                      },
                                      {
                                        "$ref": "request.model"
                                      }
                                    ]
                                  },
                                  {
                                    "$eq": [
                                      {
                                        "$ref": "price.duration"
                                      },
                                      {
                                        "$if": {
                                          "condition": {
                                            "$eq": [
                                              {
                                                "$ref": "request.duration"
                                              },
                                              0
                                            ]
                                          },
                                          "then": {
                                            "$if": {
                                              "condition": {
                                                "$eq": [
                                                  {
                                                    "$ref": "request.model"
                                                  },
                                                  "seedance2.5"
                                                ]
                                              },
                                              "then": 30,
                                              "else": 5
                                            }
                                          },
                                          "else": {
                                            "$ref": "request.duration"
                                          }
                                        }
                                      }
                                    ]
                                  }
                                ]
                              }
                            }
                          },
                          "as": "price",
                          "in": {
                            "$ref": "price.credits"
                          }
                        }
                      }
                    },
                    {
                      "$if": {
                        "condition": {
                          "$and": [
                            {
                              "$lte": [
                                {
                                  "$if": {
                                    "condition": {
                                      "$eq": [
                                        {
                                          "$ref": "request.duration"
                                        },
                                        0
                                      ]
                                    },
                                    "then": {
                                      "$if": {
                                        "condition": {
                                          "$eq": [
                                            {
                                              "$ref": "request.model"
                                            },
                                            "seedance2.5"
                                          ]
                                        },
                                        "then": 30,
                                        "else": 5
                                      }
                                    },
                                    "else": {
                                      "$ref": "request.duration"
                                    }
                                  }
                                },
                                15
                              ]
                            },
                            {
                              "$gt": [
                                {
                                  "$add": [
                                    {
                                      "$add": [
                                        {
                                          "$len": {
                                            "$ref": "request.images"
                                          }
                                        },
                                        {
                                          "$len": {
                                            "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                                          }
                                        }
                                      ]
                                    },
                                    {
                                      "$len": {
                                        "$ref": "request.videos"
                                      }
                                    }
                                  ]
                                },
                                0
                              ]
                            },
                            {
                              "$coalesce": [
                                {
                                  "$ref": "request.providerOptions.vinted-seedance-video.native_face"
                                },
                                true
                              ]
                            }
                          ]
                        },
                        "then": {
                          "$coalesce": [
                            {
                              "$ref": "response.native_face_surcharge_credits"
                            },
                            0
                          ]
                        },
                        "else": 0
                      }
                    }
                  ]
                }
              },
              "validations": [
                {
                  "assert": {
                    "$matches": [
                      {
                        "$first": {
                          "$map": {
                            "from": {
                              "$filter": {
                                "from": {
                                  "$ref": "response.items"
                                },
                                "as": "price",
                                "where": {
                                  "$and": [
                                    {
                                      "$eq": [
                                        {
                                          "$ref": "price.model"
                                        },
                                        {
                                          "$ref": "request.model"
                                        }
                                      ]
                                    },
                                    {
                                      "$eq": [
                                        {
                                          "$ref": "price.duration"
                                        },
                                        {
                                          "$if": {
                                            "condition": {
                                              "$eq": [
                                                {
                                                  "$ref": "request.duration"
                                                },
                                                0
                                              ]
                                            },
                                            "then": {
                                              "$if": {
                                                "condition": {
                                                  "$eq": [
                                                    {
                                                      "$ref": "request.model"
                                                    },
                                                    "seedance2.5"
                                                  ]
                                                },
                                                "then": 30,
                                                "else": 5
                                              }
                                            },
                                            "else": {
                                              "$ref": "request.duration"
                                            }
                                          }
                                        }
                                      ]
                                    }
                                  ]
                                }
                              }
                            },
                            "as": "price",
                            "in": {
                              "$ref": "price.credits"
                            }
                          }
                        }
                      },
                      "^[1-9][0-9]*$"
                    ]
                  },
                  "message": "当前账号没有所选模型和时长的有效整数价格"
                },
                {
                  "assert": {
                    "$if": {
                      "condition": {
                        "$and": [
                          {
                            "$lte": [
                              {
                                "$if": {
                                  "condition": {
                                    "$eq": [
                                      {
                                        "$ref": "request.duration"
                                      },
                                      0
                                    ]
                                  },
                                  "then": {
                                    "$if": {
                                      "condition": {
                                        "$eq": [
                                          {
                                            "$ref": "request.model"
                                          },
                                          "seedance2.5"
                                        ]
                                      },
                                      "then": 30,
                                      "else": 5
                                    }
                                  },
                                  "else": {
                                    "$ref": "request.duration"
                                  }
                                }
                              },
                              15
                            ]
                          },
                          {
                            "$gt": [
                              {
                                "$add": [
                                  {
                                    "$add": [
                                      {
                                        "$len": {
                                          "$ref": "request.images"
                                        }
                                      },
                                      {
                                        "$len": {
                                          "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                                        }
                                      }
                                    ]
                                  },
                                  {
                                    "$len": {
                                      "$ref": "request.videos"
                                    }
                                  }
                                ]
                              },
                              0
                            ]
                          },
                          {
                            "$coalesce": [
                              {
                                "$ref": "request.providerOptions.vinted-seedance-video.native_face"
                              },
                              true
                            ]
                          }
                        ]
                      },
                      "then": {
                        "$matches": [
                          {
                            "$ref": "response.native_face_surcharge_credits"
                          },
                          "^(0|[1-9][0-9]*)$"
                        ]
                      },
                      "else": true
                    }
                  },
                  "message": "上游未返回有效的原生人脸附加费，无法确认本次报价"
                }
              ],
              "message": {
                "$concat": [
                  "上游本次报价：",
                  {
                    "$add": [
                      {
                        "$first": {
                          "$map": {
                            "from": {
                              "$filter": {
                                "from": {
                                  "$ref": "response.items"
                                },
                                "as": "price",
                                "where": {
                                  "$and": [
                                    {
                                      "$eq": [
                                        {
                                          "$ref": "price.model"
                                        },
                                        {
                                          "$ref": "request.model"
                                        }
                                      ]
                                    },
                                    {
                                      "$eq": [
                                        {
                                          "$ref": "price.duration"
                                        },
                                        {
                                          "$if": {
                                            "condition": {
                                              "$eq": [
                                                {
                                                  "$ref": "request.duration"
                                                },
                                                0
                                              ]
                                            },
                                            "then": {
                                              "$if": {
                                                "condition": {
                                                  "$eq": [
                                                    {
                                                      "$ref": "request.model"
                                                    },
                                                    "seedance2.5"
                                                  ]
                                                },
                                                "then": 30,
                                                "else": 5
                                              }
                                            },
                                            "else": {
                                              "$ref": "request.duration"
                                            }
                                          }
                                        }
                                      ]
                                    }
                                  ]
                                }
                              }
                            },
                            "as": "price",
                            "in": {
                              "$ref": "price.credits"
                            }
                          }
                        }
                      },
                      {
                        "$if": {
                          "condition": {
                            "$and": [
                              {
                                "$lte": [
                                  {
                                    "$if": {
                                      "condition": {
                                        "$eq": [
                                          {
                                            "$ref": "request.duration"
                                          },
                                          0
                                        ]
                                      },
                                      "then": {
                                        "$if": {
                                          "condition": {
                                            "$eq": [
                                              {
                                                "$ref": "request.model"
                                              },
                                              "seedance2.5"
                                            ]
                                          },
                                          "then": 30,
                                          "else": 5
                                        }
                                      },
                                      "else": {
                                        "$ref": "request.duration"
                                      }
                                    }
                                  },
                                  15
                                ]
                              },
                              {
                                "$gt": [
                                  {
                                    "$add": [
                                      {
                                        "$add": [
                                          {
                                            "$len": {
                                              "$ref": "request.images"
                                            }
                                          },
                                          {
                                            "$len": {
                                              "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                                            }
                                          }
                                        ]
                                      },
                                      {
                                        "$len": {
                                          "$ref": "request.videos"
                                        }
                                      }
                                    ]
                                  },
                                  0
                                ]
                              },
                              {
                                "$coalesce": [
                                  {
                                    "$ref": "request.providerOptions.vinted-seedance-video.native_face"
                                  },
                                  true
                                ]
                              }
                            ]
                          },
                          "then": {
                            "$coalesce": [
                              {
                                "$ref": "response.native_face_surcharge_credits"
                              },
                              0
                            ]
                          },
                          "else": 0
                        }
                      }
                    ]
                  },
                  " 积分（包含原生人脸附加费）；最终以账单为准"
                ]
              }
            },
            {
              "id": "balance",
              "operation": {
                "method": "GET",
                "path": "/v1/balance"
              },
              "value": {
                "$ref": "response.balance"
              },
              "validations": [
                {
                  "assert": {
                    "$gte": [
                      {
                        "$ref": "response.balance"
                      },
                      {
                        "$ref": "request.extra.preparation.pricing.credits"
                      }
                    ]
                  },
                  "message": "上游账户积分不足，请充值后恢复原任务"
                }
              ],
              "message": {
                "$concat": [
                  "上游当前余额：",
                  {
                    "$ref": "response.balance"
                  },
                  " 积分"
                ]
              }
            },
            {
              "id": "images",
              "source": "images",
              "operation": {
                "method": "POST",
                "path": "/v1/files",
                "contentType": "application/json",
                "body": {
                  "image_b64": {
                    "$ref": "request.extra.prepareItem.dataUrl"
                  }
                }
              },
              "value": {
                "$ref": "response.image_id"
              },
              "validations": [
                {
                  "assert": {
                    "$matches": [
                      {
                        "$ref": "response.image_id"
                      },
                      "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"
                    ]
                  },
                  "message": "上游图片上传未返回有效 ID，已停止生成"
                }
              ]
            }
          ]
        },
        "create": {
          "method": "POST",
          "path": "/v1/videos",
          "contentType": "application/json",
          "body": {
            "model": {
              "$ref": "request.model"
            },
            "prompt": {
              "$ref": "request.prompt"
            },
            "duration": {
              "$if": {
                "condition": {
                  "$eq": [
                    {
                      "$ref": "request.duration"
                    },
                    0
                  ]
                },
                "then": {
                  "$if": {
                    "condition": {
                      "$eq": [
                        {
                          "$ref": "request.model"
                        },
                        "seedance2.5"
                      ]
                    },
                    "then": 30,
                    "else": 5
                  }
                },
                "else": {
                  "$ref": "request.duration"
                }
              }
            },
            "ratio": {
              "$omitEmpty": {
                "$ref": "request.aspectRatio"
              }
            },
            "resolution": {
              "$coalesce": [
                {
                  "$ref": "request.resolution"
                },
                "720p"
              ]
            },
            "camera_movement": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.vinted-seedance-video.camera_movement"
              }
            },
            "native_face": {
              "$ref": "request.providerOptions.vinted-seedance-video.native_face"
            },
            "image_ids": {
              "$omitEmpty": {
                "$concatArrays": [
                  {
                    "$ref": "request.providerOptions.vinted-seedance-video.image_ids"
                  },
                  {
                    "$ref": "request.extra.preparation.images"
                  }
                ]
              }
            },
            "video_b64s": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$sortByOrder": {
                      "$ref": "request.videos"
                    }
                  },
                  "as": "media",
                  "in": {
                    "$ref": "media.dataUrl"
                  }
                }
              }
            },
            "audio_b64s": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$sortByOrder": {
                      "$ref": "request.audios"
                    }
                  },
                  "as": "media",
                  "in": {
                    "$ref": "media.dataUrl"
                  }
                }
              }
            }
          },
          "headers": {
            "Idempotency-Key": {
              "$ref": "request.extra.idempotencyKey"
            },
            "User-Agent": "vinted-video-client/1.0"
          }
        },
        "poll": {
          "method": "GET",
          "path": "/v1/videos/{{taskId}}",
          "headers": {
            "User-Agent": "vinted-video-client/1.0"
          }
        },
        "result": {
          "method": "GET",
          "path": "/v1/videos/{{taskId}}/signed_url",
          "query": {
            "download": "1"
          },
          "headers": {
            "User-Agent": "vinted-video-client/1.0"
          }
        },
        "response": {
          "taskId": {
            "$coalesce": [
              {
                "$ref": "response.id"
              },
              {
                "$ref": "taskId"
              }
            ]
          },
          "status": {
            "$if": {
              "condition": {
                "$eq": [
                  {
                    "$ref": "response.quality"
                  },
                  "original"
                ]
              },
              "then": "succeeded",
              "else": {
                "$coalesce": [
                  {
                    "$ref": "response.status"
                  },
                  "failed"
                ]
              }
            }
          },
          "videos": {
            "$if": {
              "condition": {
                "$eq": [
                  {
                    "$ref": "response.quality"
                  },
                  "original"
                ]
              },
              "then": {
                "$ref": "response.url"
              },
              "else": null
            }
          },
          "message": {
            "$coalesce": [
              {
                "$ref": "response.error.message"
              },
              {
                "$ref": "response.error"
              },
              {
                "$ref": "response.message"
              },
              "原片签名响应未确认 quality=original 或未提供有效地址，请恢复原任务"
            ]
          },
          "errorPaths": [
            "error"
          ],
          "resultEphemeral": true
        }
      }
    ]
  }
}
```
<!-- YINGCE_MANIFEST_CONTRACT_END -->
