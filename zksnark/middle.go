package main

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// PublicInput 定义公共输入
type PublicInput struct {
	RootHash frontend.Variable
}

func (circuit *PublicInput) Define(api frontend.API) error {
	return nil
}

// Setup 生成证明系统的公共参数
func Setup() (plonk.ProvingKey, plonk.VerifyingKey, constraint.ConstraintSystem, error) {
	// 创建电路实例
	circuit := &NonInclusionCircuit{}

	// 编译电路
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return nil, nil, nil, err
	}

	// 生成 SRS (Structured Reference String)
	srs := kzg.NewSRS(ecc.BN254)

	// 生成证明密钥和验证密钥
	pk, vk, err := plonk.Setup(ccs, srs, srs)
	if err != nil {
		return nil, nil, nil, err
	}

	return pk, vk, ccs, nil
}

// Prove 生成证明
func Prove(pk plonk.ProvingKey, ccs constraint.ConstraintSystem, rootHash, keyPath frontend.Variable, path, helper, branchIndex []frontend.Variable) (plonk.Proof, error) {
	// 创建电路实例并设置输入
	circuit := &NonInclusionCircuit{
		RootHash:    rootHash,
		Path:        path,
		Helper:      helper,
		KeyPath:     keyPath,
		BranchIndex: branchIndex,
	}

	// 创建 witness
	w, err := frontend.NewWitness(circuit, ecc.BN254.ScalarField())
	if err != nil {
		return nil, err
	}

	// 生成证明
	proof, err := plonk.Prove(ccs, pk, w)
	if err != nil {
		return nil, err
	}

	return proof, nil
}

// Verify 验证证明
func Verify(vk plonk.VerifyingKey, proof plonk.Proof, rootHash frontend.Variable) error {
	// 创建公共输入
	publicInput := &PublicInput{
		RootHash: rootHash,
	}

	publicWitness, err := frontend.NewWitness(publicInput, ecc.BN254.ScalarField())
	if err != nil {
		return err
	}

	// 验证证明
	err = plonk.Verify(proof, vk, publicWitness)
	if err != nil {
		return err
	}

	return nil
}

// 示例使用
func Example() {
	// 1. 设置阶段
	pk, vk, ccs, err := Setup()
	if err != nil {
		panic(err)
	}

	// 2. 生成证明
	rootHash := frontend.Variable(0)     // 示例值
	keyPath := frontend.Variable(0)      // 示例值
	path := []frontend.Variable{}        // 示例值
	helper := []frontend.Variable{}      // 示例值
	branchIndex := []frontend.Variable{} // 示例值

	proof, err := Prove(pk, ccs, rootHash, keyPath, path, helper, branchIndex)
	if err != nil {
		panic(err)
	}

	// 3. 验证证明
	err = Verify(vk, proof, rootHash)
	if err != nil {
		panic(err)
	}
}
