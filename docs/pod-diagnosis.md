# AI-powered Pod Diagnosis

The **AI-powered Pod Diagnosis** feature automatically analyzes the health and status of Kubernetes Pods. It generates structured prompts based on observed behaviors and logs to support large language model (LLM)-based reasoning and troubleshooting.

This feature helps users pinpoint issues in Pods by combining failure conditions, historical warnings, and runtime logs into a unified diagnosis report.

---

## Architecture and Workflow

The diagnosis process follows these key steps:

### 1. Locate the Pod Object

* The target `Pod` is identified using its namespace and name.
* The Pod is retrieved from the Kubernetes API and validated.

### 2. Analyze Pod Status

Several categories of data are extracted:

#### **Failures**

* Failure messages are extracted under the following conditions:

  * If the Pod is in `Pending` phase and the scheduling reason is `Unschedulable`.
  * If any init container fails.
  * If any container is in a failing state (e.g., `CrashLoopBackOff`, `CreateContainerError`, unhealthy readiness probes, abnormal terminations).
* These are marked as `Failures`.

#### **Warnings**

* The system fetches all related `Events` for the Pod using Prometheus or Kubernetes API.
* These events are included as `Warnings`.

#### **Infos**

* Container logs are included as *Info* context to support runtime-level diagnostics.
* However, log fetching is conditional—only performed when the Pod meets specific failure patterns.

The decision is made by the `shouldFetchLog(pod)` function, which returns `true` if **either** of the following is detected:

1. **Terminated Containers with Errors:**

   * If any init or main container has exited with a non-zero exit code (`Terminated.ExitCode != 0`).

2. **CrashLoopBackOff States:**

   * If any container is stuck in a `Waiting` state with reason `CrashLoopBackOff`.

These conditions help ensure that logs are only collected when they are likely to provide useful insights into failure root causes, thus avoiding unnecessary overhead.

The following diagram illustrates the diagnosis logic for a Pod:

![Pod Diagnosis Flow](../docs/assets/pod-diagnosis-flow.png)

---

## AI Prompt Construction

After collecting the data, the system builds a structured prompt using a fixed template. Example template:

```text
You are a helpful Kubernetes cluster failure diagnosis expert. Please analyze the following symptoms and respond in Chinese.

Abnormal information: --- {{.ErrorInfo}} ---
Historical Pod warning events (use if helpful): --- {{.EventInfo}} ---
Pod logs (use if helpful): --- {{.LogInfo}} ---

Please respond with the following format (not exceeding 1000 characters):

Healthy: {Yes or No}
Error: {Explain the problem}
Solution: {Step-by-step recommended solution}
```

* The prompt includes:

  * `ErrorInfo`: extracted failure messages.
  * `EventInfo`: warning events.
  * `LogInfo`: container logs.
* This enables the LLM to infer root causes with high-level understanding and generate actionable outputs.

---

## Example Use Case: Diagnosing a Pod with Permission Issues

This example shows how to diagnose a problematic Pod using the `AegisDiagnosis` CRD.

### Step 1: Apply the Diagnosis CR

```bash
kubectl apply -f diagnosis-pod.yaml
```

```yaml
# diagnosis-pod.yaml
apiVersion: aegis.io/v1alpha1
kind: AegisDiagnosis
metadata:
  name: diagnose-pod
  namespace: monitoring
spec:
  object:
    kind: Pod
    name: workflow-controller-xxxxx
    namespace: scitix-system
```

### Step 2: Watch Diagnosis Execution

```bash
kubectl get -f diagnosis-pod.yaml --watch
```

Once the task completes, you should see the phase change to `Completed`.

### Step 3: Inspect the Result

```bash
kubectl describe -n monitoring aegisdiagnosises.aegis.io diagnose-pod
```

### Sample Output

```
Status:
  Phase: Completed
  Explain: Healthy: No
  Error: The container attempted to register a watch on a ConfigMap during startup but failed due to insufficient permissions.
         Error message: "configmaps 'workflow-controller-configmap' is forbidden: User 'system:serviceaccount:...:argo' cannot get resource 'configmaps' in the namespace 'scitix-system'."

  Result:
    Failures:
      - the last termination reason is Error container=workflow-controller
    Infos:
      [pod logs]
      - ... Failed to register watch for controller config map ...
    Warnings:
      - BackOff restarting failed container (count 1930)
```

### Suggested Solution by AI

1. **Check RBAC Permissions**
   Confirm whether the service account (`argo`) has access to `configmaps` in the namespace. Use:

   ```bash
   kubectl get rolebinding -n scitix-system
   kubectl get clusterrolebinding
   ```

2. **Create Role & RoleBinding (if needed)**
   If missing, create them with:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     name: argo-configmaps-reader
     namespace: scitix-system
   rules:
   - apiGroups: [""]
     resources: ["configmaps"]
     verbs: ["get", "watch", "list"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: argo-configmaps-reader-binding
     namespace: scitix-system
   subjects:
   - kind: ServiceAccount
     name: argo
     namespace: scitix-system
   roleRef:
     kind: Role
     name: argo-configmaps-reader
     apiGroup: rbac.authorization.k8s.io
   ```

3. **Redeploy the Pod**
   Delete the failing Pod so it can restart with updated permissions:

   ```bash
   kubectl delete pod workflow-controller-xxxxx -n scitix-system
   ```

4. **Monitor Logs and Status**
   Ensure the new Pod starts correctly and the permission issue is resolved.
