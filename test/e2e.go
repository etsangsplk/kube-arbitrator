/*
Copyright 2017 The Kubernetes Authors.

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

package test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/algorithm"
)

var _ = Describe("E2E Test", func() {
	It("Schedule Job with SchedulerName", func() {
		context := initTestContext()
		defer cleanupTestContext(context)
		rep := clusterSize(context, oneCPU)
		job := createQueueJobWithScheduler(context, "kar-scheduler", "qj-1", 2, rep, "busybox", oneCPU)
		err := waitJobReady(context, job.Name)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Schedule Job", func() {
		context := initTestContext()
		defer cleanupTestContext(context)
		rep := clusterSize(context, oneCPU)
		job := createJob(context, "qj-1", 2, rep, "busybox", oneCPU, nil)
		err := waitJobReady(context, job.Name)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Schedule Multiple Jobs", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		rep := clusterSize(context, oneCPU)

		job1 := createJob(context, "qj-1", 2, rep, "busybox", oneCPU, nil)
		qj2 := createJob(context, "qj-2", 2, rep, "busybox", oneCPU, nil)
		qj3 := createJob(context, "qj-3", 2, rep, "busybox", oneCPU, nil)

		err := waitJobReady(context, job1.Name)
		Expect(err).NotTo(HaveOccurred())

		err = waitJobReady(context, qj2.Name)
		Expect(err).NotTo(HaveOccurred())

		err = waitJobReady(context, qj3.Name)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Gang scheduling", func() {
		context := initTestContext()
		defer cleanupTestContext(context)
		rep := clusterSize(context, oneCPU)/2 + 1

		replicaset := createReplicaSet(context, "rs-1", rep, "nginx", oneCPU)
		err := waitReplicaSetReady(context, replicaset.Name)
		Expect(err).NotTo(HaveOccurred())

		job := createJob(context, "gang-qj", rep, rep, "busybox", oneCPU, nil)
		err = waitJobNotReady(context, job.Name)
		Expect(err).NotTo(HaveOccurred())

		err = deleteReplicaSet(context, replicaset.Name)
		Expect(err).NotTo(HaveOccurred())

		err = waitJobReady(context, job.Name)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Preemption", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		slot := oneCPU
		rep := clusterSize(context, slot)

		job1 := createJob(context, "preemptee-qj", 1, rep, "nginx", slot, nil)
		err := waitTasksReady(context, job1.Name, int(rep))
		Expect(err).NotTo(HaveOccurred())

		job2 := createJob(context, "preemptor-qj", 1, rep, "nginx", slot, nil)
		err = waitTasksReady(context, job2.Name, int(rep)/2)
		Expect(err).NotTo(HaveOccurred())

		err = waitTasksReady(context, job1.Name, int(rep)/2)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Multiple Preemption", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		slot := oneCPU
		rep := clusterSize(context, slot)

		job1 := createJob(context, "preemptee-qj", 1, rep, "nginx", slot, nil)
		err := waitTasksReady(context, job1.Name, int(rep))
		Expect(err).NotTo(HaveOccurred())

		job2 := createJob(context, "preemptor-qj", 1, rep, "nginx", slot, nil)
		Expect(err).NotTo(HaveOccurred())

		job3 := createJob(context, "preemptor-qj2", 1, rep, "nginx", slot, nil)
		Expect(err).NotTo(HaveOccurred())

		err = waitTasksReady(context, job1.Name, int(rep)/3)
		Expect(err).NotTo(HaveOccurred())

		err = waitTasksReady(context, job2.Name, int(rep)/3)
		Expect(err).NotTo(HaveOccurred())

		err = waitTasksReady(context, job3.Name, int(rep)/3)
		Expect(err).NotTo(HaveOccurred())
	})

	It("TaskPriority", func() {
		Skip("Skip TaskPriority test because of dind setting.")
		context := initTestContext()
		defer cleanupTestContext(context)

		slot := oneCPU
		rep := clusterSize(context, slot)

		replicaset := createReplicaSet(context, "rs-1", rep/2, "nginx", slot)
		err := waitReplicaSetReady(context, replicaset.Name)
		Expect(err).NotTo(HaveOccurred())

		ts := []taskSpec{
			{
				name: "worker",
				img:  "nginx",
				pri:  workerPriority,
				rep:  rep,
				req:  slot,
			},
			{
				name: "master",
				img:  "nginx",
				pri:  masterPriority,
				rep:  1,
				req:  slot,
			},
		}

		job := createJobEx(context, "multi-pod-qj", rep/2, ts)

		expectedTasks := map[string]int32{"master": 1, "worker": rep/2 - 1}
		err = waitTasksReadyEx(context, job.Name, expectedTasks)
		Expect(err).NotTo(HaveOccurred())
	})

	It("NodeAffinity", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		slot := oneCPU
		nodeName, rep := computeNode(context, oneCPU)
		Expect(rep).NotTo(Equal(0))

		affinity := &v1.Affinity{
			NodeAffinity: &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchFields: []v1.NodeSelectorRequirement{
								{
									Key:      algorithm.NodeFieldSelectorKeyNodeName,
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{nodeName},
								},
							},
						},
					},
				},
			},
		}

		job := createJob(context, "queuejob", rep, rep, "nginx", slot, affinity)
		err := waitJobReady(context, job.Name)
		Expect(err).NotTo(HaveOccurred())

		pods := getPodOfJob(context, "queuejob")
		for _, pod := range pods {
			Expect(pod.Spec.NodeName).To(Equal(nodeName))
		}
	})
})
