# OpenCompass LLM Evaluation Framework Integration Guide

## Overview

OpenCompass is an open-source LLM evaluation platform developed by Shanghai AI Lab. It provides comprehensive support for evaluating large language models across 100+ datasets and multiple model types including Llama3, Mistral, InternLM2, GPT-4, Claude, Qwen, GLM, and more.

**Repository**: https://github.com/open-compass/opencompass  
**Documentation**: https://opencompass.readthedocs.io  
**PyPI Package**: `opencompass`

---

## 1. Architecture & Components

### 1.1 Core Architecture

OpenCompass follows a layered architecture:

| Layer | Components | Description |
|-------|-----------|-------------|
| **Model Layer** | Base Model, Chat Model | Primary model categories for evaluation |
| **Capability Layer** | Language, Knowledge, Reasoning, Understanding, Safety | Evaluation dimensions |
| **Method Layer** | Objective Evaluation, Subjective Evaluation | Evaluation methodologies |
| **Tool Layer** | Distributed Evaluation, Prompt Engineering, Leaderboard | Supporting tools |

### 1.2 Core Components

- **Partitioner**: Task partition strategies (NaivePartitioner, SizePartitioner)
- **Runner**: Execution backends (LocalRunner, SlurmRunner, DLCRunner)
- **Task**: Fundamental execution units (OpenICLInferTask, OpenICLEvalTask)
- **Inferencer**: Inference methods (PPLInferencer, GenInferencer)
- **Evaluator**: Metric calculators (AccEvaluator, etc.)

### 1.3 CLI vs Python API

**CLI (Primary Interface)**:
```bash
# Basic evaluation via CLI
python run.py --datasets siqa_gen winograd_ppl \
    --hf-type base \
    --hf-path facebook/opt-125m

# Using preset configurations
python run.py --models hf_opt_125m hf_opt_350m \
    --datasets siqa_gen winograd_ppl
```

**Python API (via MMEngine Config)**:
```python
from mmengine.config import Config
cfg = Config.fromfile('./configs/eval_demo.py')

# Programmatic task execution
from opencompass.tasks import OpenICLInferTask
task = OpenICLInferTask(cfg)
task.run()
```

---

## 2. Integration Points

### 2.1 Triggering Evaluations

**Method 1: Command Line (Recommended for cloud-native integration)**

```bash
# Basic evaluation
python run.py --datasets <dataset_names> \
    --models <model_config_names>

# With custom HuggingFace model
python run.py --datasets siqa_gen \
    --hf-type base \
    --hf-path facebook/opt-125m \
    --max-seq-len 2048 \
    --max-out-len 100 \
    --batch-size 16 \
    --hf-num-gpus 1

# With execution mode control
python run.py <config_file.py> \
    --max-partition-size 2000 \
    --max-num-workers 32
```

**Method 2: Subprocess Integration**

```python
import subprocess
import json

def run_evaluation(config_path, output_dir):
    result = subprocess.run(
        ['python', 'run.py', config_path, 
         '--work-dir', output_dir],
        capture_output=True,
        text=True
    )
    return result.stdout, result.stderr
```

**Method 3: Direct Python API**

```python
from opencompass.partitioners import SizePartitioner
from opencompass.runners import LocalRunner
from opencompass.tasks import OpenICLInferTask

# Configure and run programmatically
infer = dict(
    partitioner=dict(type=SizePartitioner, max_task_size=5000),
    runner=dict(
        type=LocalRunner,
        max_num_workers=16,
        task=dict(type=OpenICLInferTask)
    )
)
```

### 2.2 Passing Parameters

Parameters can be passed through:
1. **Configuration files** (Python `.py` format)
2. **Command-line arguments**
3. **Environment variables**

```bash
# Environment variables
export DATASET_SOURCE=ModelScope  # For ModelScope datasets
export CUDA_VISIBLE_DEVICES=0,1,2,3  # GPU selection
```

### 2.3 Capturing Results

Results are stored in structured output directories:

```
outputs/default/
├── 20230220_183030     # Timestamp-based folder
│   ├── configs         # Dumped config files
│   ├── logs            # Log files for inference/evaluation
│   │   ├── eval
│   │   └── infer
│   ├── predictions     # Model predictions (JSON)
│   ├── results         # Evaluation scores (JSON)
│   └── summary         # Summarized results (CSV, TXT)
```

**Reading Results Programmatically**:

```python
import json
import pandas as pd

# Read JSON results
with open('outputs/default/<timestamp>/results/<dataset>/<model>.json') as f:
    results = json.load(f)

# Read CSV summary
summary = pd.read_csv('outputs/default/<timestamp>/summary/summary.csv')
```

---

## 3. Configuration

### 3.1 Configuration File Format

OpenCompass uses Python-style configuration files based on MMEngine:

```python
# configs/eval_demo.py
from mmengine.config import read_base

with read_base():
    from .datasets.piqa.piqa_ppl import piqa_datasets
    from .datasets.siqa.siqa_gen import siqa_datasets

datasets = [*piqa_datasets, *siqa_datasets]

from opencompass.models import HuggingFaceCausalLM

models = [
    dict(
        type=HuggingFaceCausalLM,
        path='huggyllama/llama-7b',
        tokenizer_path='huggyllama/llama-7b',
        max_seq_len=2048,
        max_out_len=100,
        batch_size=16,
        run_cfg=dict(num_gpus=1),
    )
]
```

### 3.2 Key Configuration Fields

| Field | Description | Example |
|-------|-------------|---------|
| `models` | List of model configurations | `[dict(type=HuggingFaceCausalLM, path='...')]` |
| `datasets` | List of dataset configurations | `[dict(type=HFDataset, path='piqa')]` |
| `infer` | Inference execution config | `dict(partitioner=..., runner=...)` |
| `eval` | Evaluation execution config | `dict(partitioner=..., runner=...)` |
| `run_cfg` | Resource requirements | `dict(num_gpus=8, num_procs=1)` |

### 3.3 Dataset Configuration Example

```python
# Dataset config structure
piqa_reader_cfg = dict(
    input_columns=['goal', 'sol1', 'sol2'],
    output_column='label',
    test_split='validation',
)

piqa_infer_cfg = dict(
    prompt_template=dict(
        type=PromptTemplate,
        template={
            0: 'The following makes sense: \nQ: {goal}\nA: {sol1}\n',
            1: 'The following makes sense: \nQ: {goal}\nA: {sol2}\n'
        }),
    retriever=dict(type=ZeroRetriever),
    inferencer=dict(type=PPLInferencer)
)

piqa_eval_cfg = dict(evaluator=dict(type=AccEvaluator))
```

---

## 4. Dependencies

### 4.1 Python Dependencies

**Core Installation**:
```bash
pip install -U opencompass
```

**Additional Options**:
```bash
# Full installation (more datasets)
pip install "opencompass[full]"

# API models support
pip install "opencompass[api]"

# Inference backends
pip install "opencompass[lmdeploy]"
pip install "opencompass[vllm]"
```

**Key Dependencies**:
- `pytorch >= 1.13`
- `mmengine` (configuration management)
- `transformers` (HuggingFace models)
- `huggingface-hub` (model loading)
- `datasets` (HuggingFace datasets)

### 4.2 API Model Dependencies

| Model | Package |
|-------|---------|
| OpenAI (GPT-4, GPT-3.5) | `openai` |
| Claude | `anthropic` |
| Qwen | `dashscope` |
| ByteDance Volcano Engine | `volcengine-python-sdk` |

### 4.3 System Dependencies

- Python 3.10+ recommended
- CUDA-compatible GPU for local model inference
- Slurm cluster for distributed evaluation (optional)

---

## 5. Output Formats

### 5.1 File Formats

| Format | Location | Content |
|--------|----------|---------|
| **JSON** | `predictions/`, `results/` | Detailed predictions and scores |
| **CSV** | `summary/` | Summarized evaluation table |
| **TXT** | `summary/` | Human-readable summary |
| **PKL** | `predictions/` | Serialized prediction objects |

### 5.2 JSON Result Structure

```json
{
    "predictions": [
        {"question": "...", "prediction": "...", "answer": "..."}
    ],
    "results": {
        "accuracy": 0.85,
        "total": 1000
    },
    "cfg": {
        "models": {...},
        "datasets": {...}
    }
}
```

### 5.3 Data Station Persistence

```bash
# Store results to external path
opencompass ... -sp '/your_path'

# Read existing results
opencompass ... -sp '/your_path' --read-from-station

# Overwrite existing results
opencompass ... -sp '/your_path' --station-overwrite
```

---

## 6. Model Support

### 6.1 Supported Model Types

| Type | Class | Description |
|------|-------|-------------|
| **HuggingFace Models** | `HuggingFaceCausalLM` | AutoModelForCausalLM models |
| **HuggingFace General** | `HuggingFace` | AutoModel models |
| **OpenAI API** | `OpenAI` | GPT-4, GPT-3.5, GPT-4o |
| **Claude API** | `ZhiPuAI` | Anthropic Claude |
| **MiniMax API** | `MiniMax` | ABAB-Chat |
| **XunFei API** | `XunFei` | iFlytek models |

### 6.2 Local Model Configuration

```python
from opencompass.models import HuggingFaceCausalLM

models = [
    dict(
        type=HuggingFaceCausalLM,
        path='meta-llama/Llama-2-7b-hf',
        tokenizer_path='meta-llama/Llama-2-7b-hf',
        tokenizer_kwargs=dict(
            padding_side='left',
            truncation_side='left'
        ),
        max_seq_len=4096,
        batch_padding=False,
        run_cfg=dict(num_gpus=1),
    )
]
```

### 6.3 API Model Configuration

```python
from opencompass.models import OpenAI

models = [
    dict(
        type=OpenAI,
        path='gpt-4',
        key='YOUR_OPENAI_KEY',  # or use OPENAI_API_KEY env var
        max_seq_len=2048,
        max_out_len=512,
        batch_size=1,
        run_cfg=dict(num_gpus=0),  # No GPU needed
    )
]
```

### 6.4 vLLM/LMDeploy Integration

```bash
# Install inference backend
pip install "opencompass[vllm]"

# Configure in config file
from opencompass.models import VLLM

models = [
    dict(
        type=VLLM,
        path='meta-llama/Llama-2-7b-hf',
        max_seq_len=4096,
        run_cfg=dict(num_gpus=1),
    )
]
```

---

## 7. Docker/Containerization

### 7.1 Official Docker Image

**Docker Hub**: https://hub.docker.com/r/opencsghq/opencompass

```bash
# Pull official image
docker pull opencsghq/opencompass

# Run container
docker run -it --gpus all opencsghq/opencompass
```

### 7.2 Custom Docker Setup

```dockerfile
FROM python:3.10-slim

# Install dependencies
RUN pip install opencompass[full]

# Prepare data directory
RUN mkdir -p /app/data

WORKDIR /app

# Entry point
ENTRYPOINT ["python", "run.py"]
```

### 7.3 Code Evaluation Docker

For code evaluation tasks, OpenCompass provides isolated Docker containers:

```bash
# Build code evaluator image
docker build -t code-eval-humaneval:latest .

# Run evaluation service
docker run -itd -p 5001:5001 code-eval-humaneval:latest \
    python server.py --port 5001
```

---

## 8. Scalability & Distributed Execution

### 8.1 Task Partitioning

OpenCompass supports multiple partition strategies:

| Partitioner | Description | Use Case |
|-------------|-------------|----------|
| `NaivePartitioner` | One task per model-dataset pair | Small evaluations |
| `SizePartitioner` | Split by sample count | Large datasets |

```python
from opencompass.partitioners import SizePartitioner

infer = dict(
    partitioner=dict(
        type=SizePartitioner,
        max_task_size=5000,  # Max samples per task
        gen_task_coef=20,    # Coefficient for generative tasks
    )
)
```

### 8.2 Execution Backends

**LocalRunner** (for single machine):
```python
from opencompass.runners import LocalRunner

infer = dict(
    runner=dict(
        type=LocalRunner,
        max_num_workers=16,  # Max parallel processes
        task=dict(type=OpenICLInferTask),
    )
)
```

**SlurmRunner** (for clusters):
```python
from opencompass.runners import SlurmRunner

infer = dict(
    runner=dict(
        type=SlurmRunner,
        max_num_workers=64,
        task=dict(type=OpenICLInferTask),
        retry=5,  # Retry failed tasks
    )
)
```

**DLCRunner** (Alibaba Deep Learning Center):
```python
from opencompass.runners import DLCRunner

infer = dict(
    runner=dict(
        type=DLCRunner,
        max_num_workers=16,
        aliyun_cfg=dict(
            bashrc_path="/user/.bashrc",
            conda_env_name='opencompass',
            dlc_config_path="/user/.dlc/config",
            workspace_id='ws-xxx',
            worker_image='xxx',
        ),
    )
)
```

### 8.3 CLI Distributed Execution

```bash
# Slurm cluster submission
python run.py configs/eval_demo.py \
    --slurm \
    --partition my_partition \
    --max-num-workers 64 \
    --retry 2

# Local parallel execution
python run.py configs/eval_demo.py \
    --max-partition-size 2000 \
    --max-num-workers 32
```

### 8.4 GPU Resource Management

```bash
# Limit GPU access
CUDA_VISIBLE_DEVICES=0,1,2,3 python run.py ...

# Per-model GPU allocation (in config)
run_cfg=dict(num_gpus=4, num_procs=1)
```

---

## 9. Integration Approaches for Cloud-Native Backend

### 9.1 Recommended Approach: Subprocess Wrapper

```python
import subprocess
import os
import json
from pathlib import Path

class OpenCompassEvaluator:
    def __init__(self, work_dir: str = "/tmp/opencompass_runs"):
        self.work_dir = Path(work_dir)
        self.work_dir.mkdir(exist_ok=True)
    
    def run_evaluation(
        self,
        model_path: str,
        datasets: list[str],
        output_dir: str,
        max_seq_len: int = 2048,
        max_out_len: int = 100,
        batch_size: int = 16,
        num_gpus: int = 1,
    ) -> dict:
        """Run evaluation and return results"""
        
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        run_dir = self.work_dir / timestamp
        
        cmd = [
            'python', 'run.py',
            '--datasets', *datasets,
            '--hf-type', 'base',
            '--hf-path', model_path,
            '--max-seq-len', str(max_seq_len),
            '--max-out-len', str(max_out_len),
            '--batch-size', str(batch_size),
            '--hf-num-gpus', str(num_gpus),
            '--work-dir', str(run_dir),
        ]
        
        env = os.environ.copy()
        env['CUDA_VISIBLE_DEVICES'] = ','.join(map(str, range(num_gpus)))
        
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            env=env
        )
        
        return {
            'success': result.returncode == 0,
            'output_dir': str(run_dir),
            'stdout': result.stdout,
            'stderr': result.stderr,
        }
    
    def get_results(self, run_dir: str) -> dict:
        """Parse evaluation results"""
        summary_path = Path(run_dir) / 'summary' / 'summary.csv'
        if summary_path.exists():
            import pandas as pd
            return pd.read_csv(summary_path).to_dict('records')
        return {}
```

### 9.2 Alternative: REST API Wrapper

```python
from fastapi import FastAPI, BackgroundTasks
from pydantic import BaseModel
import subprocess
import uuid

app = FastAPI()

class EvaluationRequest(BaseModel):
    model_path: str
    datasets: list[str]
    config: dict = {}

class EvaluationResponse(BaseModel):
    job_id: str
    status: str

jobs = {}

@app.post("/evaluate", response_model=EvaluationResponse)
async def create_evaluation(request: EvaluationRequest, background_tasks: BackgroundTasks):
    job_id = str(uuid.uuid4())
    jobs[job_id] = {'status': 'pending', 'results': None}
    
    background_tasks.add_task(run_evaluation_task, job_id, request)
    
    return EvaluationResponse(job_id=job_id, status='pending')

@app.get("/jobs/{job_id}")
async def get_job_status(job_id: str):
    return jobs.get(job_id, {'status': 'not_found'})

def run_evaluation_task(job_id: str, request: EvaluationRequest):
    # Execute OpenCompass evaluation
    cmd = build_command(request)
    subprocess.run(cmd)
    
    # Parse and store results
    jobs[job_id]['status'] = 'completed'
    jobs[job_id]['results'] = parse_results()
```

### 9.3 Docker Deployment

```yaml
# docker-compose.yml
version: '3.8'
services:
  opencompass-api:
    build: .
    ports:
      - "8000:8000"
    volumes:
      - ./configs:/app/configs
      - ./outputs:/app/outputs
    environment:
      - CUDA_VISIBLE_DEVICES=0,1,2,3
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

---

## 10. Limitations & Constraints

### 10.1 Known Limitations

1. **No Native REST API**: OpenCompass is primarily CLI-based; requires wrapper for API access
2. **Configuration Complexity**: Python config files require MMEngine knowledge
3. **Dataset Download**: Large datasets require manual download for some cases
4. **GPU Memory**: Large models may require significant GPU memory
5. **Task Partition**: SizePartitioner not suitable for evaluation tasks

### 10.2 Resource Requirements

| Model Size | Recommended GPUs | Memory per GPU |
|------------|-----------------|----------------|
| 7B | 1 | 16GB |
| 13B | 1-2 | 24GB |
| 70B | 4-8 | 40GB |

### 10.3 Performance Considerations

- Inference time scales with dataset size and model complexity
- Distributed execution recommended for multi-model evaluation
- Use vLLM or LMDeploy for faster inference

---

## 11. Quick Reference Commands

```bash
# Installation
pip install opencompass

# List available configs
python tools/list_configs.py

# Basic evaluation
python run.py --datasets mmlu_gen --hf-path meta-llama/Llama-2-7b-hf

# Slurm evaluation
python run.py configs/eval.py --slurm --partition gpu

# API model evaluation
python run.py --datasets mmlu_gen --model-type openai --model-path gpt-4

# Results summary
python run.py configs/eval.py -m viz  # Only visualization
```

---

## References

- Official Documentation: https://opencompass.readthedocs.io
- GitHub Repository: https://github.com/open-compass/opencompass
- PyPI Package: https://pypi.org/project/opencompass
- Leaderboard: https://opencompass.org.cn/leaderboard-llm
- MMEngine Config: https://mmengine.readthedocs.io/en/latest/advanced_tutorials/config.html
