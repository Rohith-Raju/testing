/*
Copyright (C) 2023 The Falco Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package rules

import (
	"github.com/falcosecurity/testing/pkg/run"
)

var K8SAuditRules = run.NewLocalFileAccessor(
	" k8s_audit_rules.yaml",
	"/etc/falco/k8s_audit_rules.yaml",
)

var K8SAuditAllowNamespaceFoo = run.NewStringFileAccessor(
	"allow_namespace_foo.yaml",
	`- list: allowed_namespaces
  items: [foo]
  append: true
`,
)

var K8SAuditAllowNginxContainer = run.NewStringFileAccessor(
	"allow_nginx_container.yaml",
	`- macro: allowed_k8s_containers
  condition: (ka.req.pod.containers.image.repository in (nginx))
`,
)

var K8SAuditAllowOnlyApacheContainer = run.NewStringFileAccessor(
	"allow_only_apache_container.yaml",
	`- macro: allowed_k8s_containers
  condition: (ka.req.pod.containers.image.repository in (apache))
`,
)

var K8SAuditAllowUserSomeUser = run.NewStringFileAccessor(
	"allow_user_some-user.yaml",
	`- list: allowed_k8s_users
  items: [some-user]
  append: true
`,
)

var K8SAuditDisallowKactivity = run.NewStringFileAccessor(
	"disallow_kactivity.yaml",
	`- macro: consider_activity_events
  condition: (not jevt.rawtime exists)`,
)

var K8SAuditEngineV4AllowNginxContainer = run.NewStringFileAccessor(
	"allow_nginx_container.yaml",
	`- macro: allowed_k8s_containers
  condition: (ka.req.container.image.repository=nginx)
`,
)

var K8SAuditEngineV4AllowOnlyApacheContainer = run.NewStringFileAccessor(
	"allow_only_apache_container.yaml",
	`- macro: allowed_k8s_containers
  condition: (ka.req.container.image=apache)
`,
)

var K8SAuditEngineV4K8SAuditRules = run.NewStringFileAccessor(
	"engine_v4_k8s_audit_rules.yaml",
	`
- required_engine_version: 2

# Like always_true/always_false, but works with k8s audit events
- macro: k8s_audit_always_true
  condition: (jevt.rawtime exists)

- macro: k8s_audit_never_true
  condition: (jevt.rawtime=0)

# Generally only consider audit events once the response has completed
- list: k8s_audit_stages
  items: ["ResponseComplete"]

# Generally exclude users starting with "system:"
- macro: non_system_user
  condition: (not ka.user.name startswith "system:")

# This macro selects the set of Audit Events used by the below rules.
- macro: kevt
  condition: (jevt.value[/stage] in (k8s_audit_stages))

- macro: kevt_started
  condition: (jevt.value[/stage]=ResponseStarted)

# If you wish to restrict activity to a specific set of users, override/append to this list.
- list: allowed_k8s_users
  items: ["minikube", "minikube-user", "kubelet", "kops"]

- rule: Disallowed K8s User
  desc: Detect any k8s operation by users outside of an allowed set of users.
  condition: kevt and non_system_user and not ka.user.name in (allowed_k8s_users)
  output: K8s Operation performed by user not in allowed list of users (user=%ka.user.name target=%ka.target.name/%ka.target.resource verb=%ka.verb uri=%ka.uri resp=%ka.response.code)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# In a local/user rules file, you could override this macro to
# explicitly enumerate the container images that you want to run in
# your environment. In this main falco rules file, there isn't any way
# to know all the containers that can run, so any container is
# allowed, by using the always_true macro. In the overridden macro, the condition
# would look something like (ka.req.container.image.repository=my-repo/my-image)
- macro: allowed_k8s_containers
  condition: (k8s_audit_always_true)

- macro: response_successful
  condition: (ka.response.code startswith 2)

- macro: kcreate
  condition: ka.verb=create

- macro: kmodify
  condition: (ka.verb in (create,update,patch))

- macro: kdelete
  condition: ka.verb=delete

- macro: pod
  condition: ka.target.resource=pods and not ka.target.subresource exists

- macro: pod_subresource
  condition: ka.target.resource=pods and ka.target.subresource exists

- macro: deployment
  condition: ka.target.resource=deployments

- macro: service
  condition: ka.target.resource=services

- macro: configmap
  condition: ka.target.resource=configmaps

- macro: namespace
  condition: ka.target.resource=namespaces

- macro: serviceaccount
  condition: ka.target.resource=serviceaccounts

- macro: clusterrole
  condition: ka.target.resource=clusterroles

- macro: clusterrolebinding
  condition: ka.target.resource=clusterrolebindings

- macro: role
  condition: ka.target.resource=roles

- macro: health_endpoint
  condition: ka.uri=/healthz

- rule: Create Disallowed Pod
  desc: >
    Detect an attempt to start a pod with a container image outside of a list of allowed images.
  condition: kevt and pod and kcreate and not allowed_k8s_containers
  output: Pod started with container not in allowed list (user=%ka.user.name pod=%ka.resp.name ns=%ka.target.namespace image=%ka.req.container.image)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

- rule: Create Privileged Pod
  desc: >
    Detect an attempt to start a pod with a privileged container
  condition: kevt and pod and kcreate and ka.req.container.privileged=true and not ka.req.container.image.repository in (falco_privileged_images)
  output: Pod started with privileged container (user=%ka.user.name pod=%ka.resp.name ns=%ka.target.namespace image=%ka.req.container.image)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

- macro: sensitive_vol_mount
  condition: >
    (ka.req.volume.hostpath[/proc*]=true or
     ka.req.volume.hostpath[/var/run/docker.sock]=true or
     ka.req.volume.hostpath[/]=true or
     ka.req.volume.hostpath[/etc]=true or
     ka.req.volume.hostpath[/root*]=true)

- rule: Create Sensitive Mount Pod
  desc: >
    Detect an attempt to start a pod with a volume from a sensitive host directory (i.e. /proc).
    Exceptions are made for known trusted images.
  condition: kevt and pod and kcreate and sensitive_vol_mount and not ka.req.container.image.repository in (falco_sensitive_mount_images)
  output: Pod started with sensitive mount (user=%ka.user.name pod=%ka.resp.name ns=%ka.target.namespace image=%ka.req.container.image mounts=%jevt.value[/requestObject/spec/volumes])
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# Corresponds to K8s CIS Benchmark 1.7.4
- rule: Create HostNetwork Pod
  desc: Detect an attempt to start a pod using the host network.
  condition: kevt and pod and kcreate and ka.req.container.host_network=true and not ka.req.container.image.repository in (falco_hostnetwork_images)
  output: Pod started using host network (user=%ka.user.name pod=%ka.resp.name ns=%ka.target.namespace image=%ka.req.container.image)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

- rule: Create NodePort Service
  desc: >
    Detect an attempt to start a service with a NodePort service type
  condition: kevt and service and kcreate and ka.req.service.type=NodePort
  output: NodePort Service Created (user=%ka.user.name service=%ka.target.name ns=%ka.target.namespace ports=%ka.req.service.ports)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

- macro: contains_private_credentials
  condition: >
    (ka.req.configmap.obj contains "aws_access_key_id" or
     ka.req.configmap.obj contains "aws-access-key-id" or
     ka.req.configmap.obj contains "aws_s3_access_key_id" or
     ka.req.configmap.obj contains "aws-s3-access-key-id" or
     ka.req.configmap.obj contains "password" or
     ka.req.configmap.obj contains "passphrase")

- rule: Create/Modify Configmap With Private Credentials
  desc: >
     Detect creating/modifying a configmap containing a private credential (aws key, password, etc.)
  condition: kevt and configmap and kmodify and contains_private_credentials
  output: K8s configmap with private credential (user=%ka.user.name verb=%ka.verb configmap=%ka.req.configmap.name config=%ka.req.configmap.obj)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# Corresponds to K8s CIS Benchmark, 1.1.1.
- rule: Anonymous Request Allowed
  desc: >
    Detect any request made by the anonymous user that was allowed
  condition: kevt and ka.user.name=system:anonymous and ka.auth.decision!=reject and not health_endpoint
  output: Request by anonymous user allowed (user=%ka.user.name verb=%ka.verb uri=%ka.uri reason=%ka.auth.reason))
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# Roughly corresponds to K8s CIS Benchmark, 1.1.12. In this case,
# notifies an attempt to exec/attach to a privileged container.

# Ideally, we'd add a more stringent rule that detects attaches/execs
# to a privileged pod, but that requires the engine for k8s audit
# events to be stateful, so it could know if a container named in an
# attach request was created privileged or not. For now, we have a
# less severe rule that detects attaches/execs to any pod.

- rule: Attach/Exec Pod
  desc: >
    Detect any attempt to attach/exec to a pod
  condition: kevt_started and pod_subresource and kcreate and ka.target.subresource in (exec,attach)
  output: Attach/Exec to pod (user=%ka.user.name pod=%ka.target.name ns=%ka.target.namespace action=%ka.target.subresource command=%ka.uri.param[command])
  priority: NOTICE
  source: k8s_audit
  tags: [k8s]

# In a local/user rules fie, you can append to this list to add additional allowed namespaces
- list: allowed_namespaces
  items: [kube-system, kube-public, default]

- rule: Create Disallowed Namespace
  desc: Detect any attempt to create a namespace outside of a set of known namespaces
  condition: kevt and namespace and kcreate and not ka.target.name in (allowed_namespaces)
  output: Disallowed namespace created (user=%ka.user.name ns=%ka.target.name)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# Detect any new pod created in the kube-system namespace
- rule: Pod Created in Kube Namespace
  desc: Detect any attempt to create a pod in the kube-system or kube-public namespaces
  condition: kevt and pod and kcreate and ka.target.namespace in (kube-system, kube-public)
  output: Pod created in kube namespace (user=%ka.user.name pod=%ka.resp.name ns=%ka.target.namespace image=%ka.req.container.image)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# Detect creating a service account in the kube-system/kube-public namespace
- rule: Service Account Created in Kube Namespace
  desc: Detect any attempt to create a serviceaccount in the kube-system or kube-public namespaces
  condition: kevt and serviceaccount and kcreate and ka.target.namespace in (kube-system, kube-public)
  output: Service account created in kube namespace (user=%ka.user.name serviceaccount=%ka.target.name ns=%ka.target.namespace)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# Detect any modify/delete to any ClusterRole starting with
# "system:". "system:coredns" is excluded as changes are expected in
# normal operation.
- rule: System ClusterRole Modified/Deleted
  desc: Detect any attempt to modify/delete a ClusterRole/Role starting with system
  condition: kevt and (role or clusterrole) and (kmodify or kdelete) and (ka.target.name startswith "system:") and ka.target.name!="system:coredns"
  output: System ClusterRole/Role modified or deleted (user=%ka.user.name role=%ka.target.name ns=%ka.target.namespace action=%ka.verb)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# Detect any attempt to create a ClusterRoleBinding to the cluster-admin user
# (expand this to any built-in cluster role that does "sensitive" things)
- rule: Attach to cluster-admin Role
  desc: Detect any attempt to create a ClusterRoleBinding to the cluster-admin user
  condition: kevt and clusterrolebinding and kcreate and ka.req.binding.role=cluster-admin
  output: Cluster Role Binding to cluster-admin role (user=%ka.user.name subject=%ka.req.binding.subjects)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

- rule: ClusterRole With Wildcard Created
  desc: Detect any attempt to create a Role/ClusterRole with wildcard resources or verbs
  condition: kevt and (role or clusterrole) and kcreate and (ka.req.role.rules.resources intersects ("*") or ka.req.role.rules.verbs intersects ("*"))
  output: Created Role/ClusterRole with wildcard (user=%ka.user.name role=%ka.target.name rules=%ka.req.role.rules)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

- macro: writable_verbs
  condition: >
    (ka.req.role.rules.verbs intersects (create) or
     ka.req.role.rules.verbs intersects (update) or
     ka.req.role.rules.verbs intersects (patch) or
     ka.req.role.rules.verbs intersects (delete) or
     ka.req.role.rules.verbs intersects (deletecollection))

- rule: ClusterRole With Write Privileges Created
  desc: Detect any attempt to create a Role/ClusterRole that can perform write-related actions
  condition: kevt and (role or clusterrole) and kcreate and writable_verbs
  output: Created Role/ClusterRole with write privileges (user=%ka.user.name role=%ka.target.name rules=%ka.req.role.rules)
  priority: NOTICE
  source: k8s_audit
  tags: [k8s]

- rule: ClusterRole With Pod Exec Created
  desc: Detect any attempt to create a Role/ClusterRole that can exec to pods
  condition: kevt and (role or clusterrole) and kcreate and ka.req.role.rules.resources contains "pods/exec"
  output: Created Role/ClusterRole with pod exec privileges (user=%ka.user.name role=%ka.target.name rules=%ka.req.role.rules)
  priority: WARNING
  source: k8s_audit
  tags: [k8s]

# The rules below this point are less discriminatory and generally
# represent a stream of activity for a cluster. If you wish to disable
# these events, modify the following macro.
- macro: consider_activity_events
  condition: (k8s_audit_always_true)

- macro: kactivity
  condition: (kevt and consider_activity_events)

- rule: K8s Deployment Created
  desc: Detect any attempt to create a deployment
  condition: (kactivity and kcreate and deployment and response_successful)
  output: K8s Deployment Created (user=%ka.user.name deployment=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Deployment Deleted
  desc: Detect any attempt to delete a deployment
  condition: (kactivity and kdelete and deployment and response_successful)
  output: K8s Deployment Deleted (user=%ka.user.name deployment=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Service Created
  desc: Detect any attempt to create a service
  condition: (kactivity and kcreate and service and response_successful)
  output: K8s Service Created (user=%ka.user.name service=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Service Deleted
  desc: Detect any attempt to delete a service
  condition: (kactivity and kdelete and service and response_successful)
  output: K8s Service Deleted (user=%ka.user.name service=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s ConfigMap Created
  desc: Detect any attempt to create a configmap
  condition: (kactivity and kcreate and configmap and response_successful)
  output: K8s ConfigMap Created (user=%ka.user.name configmap=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s ConfigMap Deleted
  desc: Detect any attempt to delete a configmap
  condition: (kactivity and kdelete and configmap and response_successful)
  output: K8s ConfigMap Deleted (user=%ka.user.name configmap=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Namespace Created
  desc: Detect any attempt to create a namespace
  condition: (kactivity and kcreate and namespace and response_successful)
  output: K8s Namespace Created (user=%ka.user.name namespace=%ka.target.name resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Namespace Deleted
  desc: Detect any attempt to delete a namespace
  condition: (kactivity and non_system_user and kdelete and namespace and response_successful)
  output: K8s Namespace Deleted (user=%ka.user.name namespace=%ka.target.name resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Serviceaccount Created
  desc: Detect any attempt to create a service account
  condition: (kactivity and kcreate and serviceaccount and response_successful)
  output: K8s Serviceaccount Created (user=%ka.user.name user=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Serviceaccount Deleted
  desc: Detect any attempt to delete a service account
  condition: (kactivity and kdelete and serviceaccount and response_successful)
  output: K8s Serviceaccount Deleted (user=%ka.user.name user=%ka.target.name ns=%ka.target.namespace resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Role/Clusterrole Created
  desc: Detect any attempt to create a cluster role/role
  condition: (kactivity and kcreate and (clusterrole or role) and response_successful)
  output: K8s Cluster Role Created (user=%ka.user.name role=%ka.target.name rules=%ka.req.role.rules resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Role/Clusterrole Deleted
  desc: Detect any attempt to delete a cluster role/role
  condition: (kactivity and kdelete and (clusterrole or role) and response_successful)
  output: K8s Cluster Role Deleted (user=%ka.user.name role=%ka.target.name resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Role/Clusterrolebinding Created
  desc: Detect any attempt to create a clusterrolebinding
  condition: (kactivity and kcreate and clusterrolebinding and response_successful)
  output: K8s Cluster Role Binding Created (user=%ka.user.name binding=%ka.target.name subjects=%ka.req.binding.subjects role=%ka.req.binding.role resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason foo=%ka.req.binding.subject.has_name[cluster-admin])
  priority: INFO
  source: k8s_audit
  tags: [k8s]

- rule: K8s Role/Clusterrolebinding Deleted
  desc: Detect any attempt to delete a clusterrolebinding
  condition: (kactivity and kdelete and clusterrolebinding and response_successful)
  output: K8s Cluster Role Binding Deleted (user=%ka.user.name binding=%ka.target.name resp=%ka.response.code decision=%ka.auth.decision reason=%ka.auth.reason)
  priority: INFO
  source: k8s_audit
  tags: [k8s]

# This rule generally matches all events, and as a result is disabled
# by default. If you wish to enable these events, modify the
# following macro.
#  condition: (jevt.rawtime exists)
- macro: consider_all_events
  condition: (k8s_audit_never_true)

- macro: kall
  condition: (kevt and consider_all_events)

- rule: All K8s Audit Events
  desc: Match all K8s Audit Events
  condition: kall
  output: K8s Audit Event received (user=%ka.user.name verb=%ka.verb uri=%ka.uri obj=%jevt.obj)
  priority: DEBUG
  source: k8s_audit
  tags: [k8s]
`,
)

var K8SAuditSingleRuleWithJsonPointer = run.NewStringFileAccessor(
	"single_rule_with_json_pointer.yaml",
	`
- rule: json_pointer_example
  desc: A rule example using JSON Pointer (RFC 6901)
  condition: jevt.value[/annotations/authorization.k8s.io~1decision] exists and not (jevt.value[/a~1~0b] exists)
  output: "JSON Pointer Test Alert"
  priority: WARNING
  source: k8s_audit
`,
)

var K8SAuditTrustNginxContainer = run.NewStringFileAccessor(
	"trust_nginx_container.yaml",
	`- list: falco_sensitive_mount_images
  items: [nginx]
  append: true

- list: falco_privileged_images
  items: [nginx]
  append: true

- list: falco_hostnetwork_images
  items: [nginx]
  append: true
`,
)
