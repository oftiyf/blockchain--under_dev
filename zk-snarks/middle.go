package main

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// Setup 生成证明系统的公共参数
func Setup() (groth16.ProvingKey, groth16.VerifyingKey, error) {
	// 创建电路实例
	circuit := &NonInclusionCircuit{}

	// 编译电路
	r1cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return nil, nil, err
	}

	// 生成证明密钥和验证密钥
	pk, vk, err := groth16.Setup(r1cs)
	if err != nil {
		return nil, nil, err
	}

	return pk, vk, nil
}

// Prove 生成证明
func Prove(pk groth16.ProvingKey, rootHash, keyPath frontend.Variable, path, helper []frontend.Variable) (groth16.Proof, error) {
	// 创建电路实例并设置输入
	circuit := &NonInclusionCircuit{
		RootHash: rootHash,
		Path:     path,
		Helper:   helper,
		KeyPath:  keyPath,
	}

	// 生成证明
	proof, err := groth16.Prove(r1cs, pk, circuit)
	if err != nil {
		return nil, err
	}

	return proof, nil
}

// Verify 验证证明
func Verify(vk groth16.VerifyingKey, proof groth16.Proof, rootHash frontend.Variable) error {
	// 创建公共输入
	publicInput := []frontend.Variable{rootHash}

	// 验证证明
	err := groth16.Verify(proof, vk, publicInput)
	if err != nil {
		return err
	}

	return nil
}

// 示例使用
func Example() {
	// 1. 设置阶段
	pk, vk, err := Setup()
	if err != nil {
		panic(err)
	}

	// 2. 生成证明
	// 这里需要提供实际的输入数据
	rootHash := frontend.Variable(0) // 示例值
	keyPath := frontend.Variable(0)  // 示例值
	path := []frontend.Variable{}    // 示例值
	helper := []frontend.Variable{}  // 示例值

	proof, err := Prove(pk, rootHash, keyPath, path, helper)
	if err != nil {
		panic(err)
	}

	// 3. 验证证明
	err = Verify(vk, proof, rootHash)
	if err != nil {
		panic(err)
	}
}
