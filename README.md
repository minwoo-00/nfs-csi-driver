# 쿠버네티스 스토리지와 CSI — 직접 구현으로 이해하는 CSI Driver

쿠버네티스 스토리지의 추상화 발전 과정과 PV 생성부터 Pod 마운트까지의 과정을 이해하는 것을 목표로 진행한 프로젝트입니다.
CSI(Container Storage Interface)를 직접 구현하고 배포하여 쿠버네티스 환경에서 직접 사용해보는 실습을 진행했습니다.

---


## 🪧쿠버네티스 스토리지 발전 과정

### 1️⃣ Pod의 Stateless 특성

Pod는 언제든지 죽고 재시작될 수 있는 임시 실행 단위입니다. Pod가 재시작되면 컨테이너 내부의 모든 데이터는 소멸되기 때문에 데이터 영속성이 중요한 서비스를 실행하는 경우 치명적인 문제가 발생합니다.


### 2️⃣ PV와 PVC — Pod와 스토리지 분리

스토리지를 pod의 lifecyle과 분리하였습니다. 또한 PV와 PVC의 등장으로 관리자와 개발자의 영역이 분리되었습니다.

```yaml
# pv.yaml (관리자의 역할)
apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
```

```yaml
# pvc.yaml (개발자의 역할)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
```

- **PV (PersistentVolume)**: 클러스터가 관리하는 실제 저장 공간
- **PVC (PersistentVolumeClaim)**: 개발자가 요청하는 스토리지 명세
- PVC 요청을 기반으로 적절한 PV를 자동 바인딩하여 Pod에 연결

 


⚠️ 그러나 PVC가 매우 많아진다면 관리자가 PV를 수동으로 미리 생성해야 하는 문제가 발생합니다. (정적 프로비저닝)


### 3️⃣ StorageClass — 동적 프로비저닝

PVC를 만들 때 StorageClass 이름을 지정해주면 StorageClass의 provisioner로 지정된 CSI Driver가 PV를 자동으로 생성합니다.

```yaml
# storageclass.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nfs-csi
provisioner: nfs.csi.seminar.dev
reclaimPolicy: Delete
volumeBindingMode: Immediate
```

- **StorageClass**: PV를 만드는 규칙을 정의한 레시피 (자체적으로 실행되지 않음)
- PVC 요청 시 실시간으로 PV를 자동 생성 (동적 프로비저닝)

---

## 💡CSI란 무엇인가

### 1️⃣ 기존 In-tree 드라이버 구조의 문제

CSI 등장 이전에는 AWS, GCP, Ceph 등 모든 스토리지 벤더의 코드가 쿠버네티스 코어에 직접 포함되어 있었습니다.

- 벤더 코드에서 장애 발생 시 쿠버네티스 전체에 영향
- 스토리지 드라이버 업데이트 주기가 k8s 릴리즈 사이클에 종속
- 쿠버네티스 코드베이스 오염 및 유지보수 어려움

### 2️⃣ CSI의 등장

**CSI(Container Storage Interface)** = 컨테이너 오케스트레이션과 스토리지 벤더 사이의 표준 인터페이스

```
Kubernetes Core  →  CSI Interface  →  AWS EBS CSI Driver
                                   →  GCP PD CSI Driver
                                   →  Ceph CSI Driver
                                   →  Azure Disk CSI Driver
```

- 벤더 코드를 쿠버네티스 밖으로 완전히 분리
- 표준 인터페이스(gRPC)만 구현하면 어떤 스토리지든 연결 가능
- 스토리지 벤더가 k8s 릴리즈와 무관하게 독립적으로 업데이트 가능
- k8s뿐만 아니라 Cloud Foundry, Nomad 등 여러 컨테이너 오케스트레이션의 표준

---

## 🗃️CSI 아키텍처 — 컴포넌트 구조

### CSI Driver의 3가지 서비스

CSI Driver는 CSI 스펙을 구현한 gRPC 서버입니다.

| 서비스 | 역할 | 주요 메서드 |
|--------|------|------------|
| **Identity** | CSI Driver의 신원을 쿠버네티스에 알림 | GetPluginInfo, GetPluginCapabilities, Probe |
| **Controller** | 스토리지 리소스 생성/삭제 (클러스터 레벨) | CreateVolume, DeleteVolume, ValidateVolumeCapabilities |
| **Node** | Pod가 실행되는 노드에서 볼륨 마운트/언마운트 | NodePublishVolume, NodeUnpublishVolume, NodeGetVolumeStats |

### 2가지 배포 형태

**Controller Plugin — Deployment**

PV 생성/삭제는 클러스터 레벨의 작업으로 클러스터 전체에서 1개만 필요합니다. 고가용성을 위해 Deployment로 배포합니다.

```
Controller Plugin (Master Node)
├── CSI Driver Container (Identity + Controller)
└── external-provisioner (Sidecar)
```

**Node Plugin — DaemonSet**

PV 마운트/언마운트는 Pod가 뜨는 노드에서 실행되는 노드 레벨 작업입니다. 모든 노드에 자동 배치하기 위해 DaemonSet으로 배포합니다.

```
Node Plugin (모든 Worker Node)
├── CSI Driver Container (Identity + Node)
└── node-driver-registrar (Sidecar)
```

### Sidecar 컨테이너 — "쿠버네티스와 CSI gRPC 사이의 번역기"

| Sidecar | 역할 |
|---------|------|
| **external-provisioner** | PVC 감지 → CSI Driver의 CreateVolume 호출 |
| **node-driver-registrar** | kubelet에 CSI Driver 소켓 경로 등록 |

Sidecar가 k8s API 로직을 담당하기 때문에 벤더는 스토리지 로직 구현에만 집중할 수 있습니다. 어떤 스토리지를 사용해도 동일한 흐름으로 디버깅이 가능합니다.

---

## ⛳PVC 생성부터 Pod 마운트까지 전체 흐름

<img width="1536" height="1024" alt="image" src="https://github.com/user-attachments/assets/7ea199a7-5565-4495-9c86-bfa7096572ef" />


**Phase 1 — PVC 생성 → PV 자동 생성**
1. external-provisioner가 PVC 감지
2. CSI Driver의 `CreateVolume` gRPC 호출
3. CSI Driver가 실제 스토리지 생성 (Storage API 호출)
4. PV 자동 생성 + PVC 바인딩 완료

**Phase 2 — Pod 스케줄링 → 마운트**

5. Pod 스케줄링 이후 kubelet이 node-driver-registrar가 등록한 소켓으로 CSI Node Plugin에 `NodePublishVolume` gRPC 호출

6. CSI Driver가 Pod의 `/data` 경로에 마운트

---

## 🛠️CSI Driver 구현

### 구현 개요

| 항목 | 내용 |
|------|------|
| 구현 언어 | Go |
| CSI Spec | v1.12.0 |
| 스토리지 백엔드 | NFS |
| 구현 서비스 | Identity / Controller / Node |
| 추가 구현 | Prometheus 메트릭 + Grafana 시각화 (CSI 스펙 외 자체 추가) |
| CI/CD | GitHub Actions → DockerHub |


### 프로젝트 구조

```
nfs-csi-driver/
├── cmd/
│   └── main.go              # 진입점, 플래그 파싱
├── pkg/
│   ├── driver/
│   │   ├── driver.go        # gRPC 서버 설정, 볼륨 카운트 초기화
│   │   ├── identity.go      # Identity Service 구현
│   │   ├── controller.go    # Controller Service 구현
│   │   └── node.go          # Node Service 구현
│   └── metrics/
│       └── metrics.go       # Prometheus 메트릭 정의 (CSI 스펙 외 자체 추가)
├── deploy/
│   ├── csidriver.yaml       # CSIDriver 오브젝트 등록
│   ├── rbac.yaml            # ServiceAccount, ClusterRole, ClusterRoleBinding
│   ├── controller.yaml      # Controller Plugin Deployment
│   ├── node.yaml            # Node Plugin DaemonSet
│   ├── storageclass.yaml    # StorageClass
│   └── podmonitor.yaml      # Prometheus PodMonitor
├── Dockerfile
└── .github/
    └── workflows/
        └── build.yml        # GitHub Actions CI/CD
```

## 🖥️모니터링

CSI 스펙에는 포함되지 않는 자체 추가 기능입니다. `NodeGetVolumeStats` 메서드 안에서 수집한 사용량 데이터를 Prometheus 메트릭으로 노출하고 Grafana로 시각화합니다.

<img width="1893" height="899" alt="image" src="https://github.com/user-attachments/assets/d4d6d612-a477-4da0-bdbd-325a66f8a5ec" />

**Grafana 대시보드 구성:**
- NFS 서버 전체 현황 (전체 용량 / 사용 중 / 남은 용량 / 사용률 게이지)
- PV별 사용량 실시간 그래프 (PV 추가/삭제 시 자동 반영)
- 볼륨 임계치 알림 상태 (80% 초과 시 경고 표시)
- 활성 볼륨 수 및 생성/삭제 카운터

### Prometheus 메트릭

| 메트릭 | 타입 | 설명 |
|--------|------|------|
| `csi_volume_used_bytes` | Gauge | PV 디렉토리 실제 사용량 (`du -sb` 방식, PV별 독립 측정) |
| `csi_volume_usage_alert` | Gauge | PVC 요청 용량의 80% 초과 시 1, 정상 시 0 |
| `csi_nfs_total_bytes` | Gauge | NFS 서버 전체 디스크 용량 |
| `csi_nfs_used_bytes` | Gauge | NFS 서버 디스크 사용량 |
| `csi_nfs_available_bytes` | Gauge | NFS 서버 디스크 남은 용량 |
| `csi_volumes_total` | Gauge | 현재 활성 PV 개수 |
| `csi_volume_operations_total` | Counter | CreateVolume / DeleteVolume 호출 횟수 |

---

## 🚩환경

| 항목 | 내용 |
|------|------|
| Kubernetes | v1.30.14 |
| 컨테이너 런타임 | containerd v1.7.28 |
| Go | 1.24 |
| CSI Spec | v1.12.0 |
| 클러스터 구성 | VirtualBox VM 3대 (마스터 1 + 워커 2) |
| NFS 서버 | 마스터 노드 (`/srv/nfs-data`) |
| 모니터링 | kube-prometheus-stack (Prometheus + Grafana) |

---

## 참고 자료

- [CSI Spec](https://github.com/container-storage-interface/spec)
- [csi-driver-host-path (쿠버네티스 레퍼런스 구현체)](https://github.com/kubernetes-csi/csi-driver-host-path)
- [쿠버네티스 CSI 공식 문서](https://kubernetes.io/docs/concepts/storage/volumes/#csi)
