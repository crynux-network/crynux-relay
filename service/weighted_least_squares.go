package service

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

func accumulateWeightedSample(xtx *mat.SymDense, xty []float64, x []float64, y, alpha float64) {
	decay := 1 - alpha
	for i := range x {
		for j := i; j < len(x); j++ {
			xtx.SetSym(i, j, decay*xtx.At(i, j)+alpha*x[i]*x[j])
		}
		xty[i] = decay*xty[i] + alpha*x[i]*y
	}
}

func fitRidgeLeastSquares(xtx *mat.SymDense, xty, initial []float64, regularization float64) ([]float64, error) {
	n := len(initial)
	if len(xty) != n || xtx.SymmetricDim() != n {
		return nil, errors.New("ridge least-squares dimensions do not match")
	}
	matrix := mat.NewSymDense(n, nil)
	yValues := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			matrix.SetSym(i, j, xtx.At(i, j))
		}
		matrix.SetSym(i, i, matrix.At(i, i)+regularization)
		yValues[i] = xty[i] + regularization*initial[i]
	}
	scale := make([]float64, n)
	for i := range scale {
		scale[i] = math.Sqrt(matrix.At(i, i))
		if scale[i] <= 0 || math.IsNaN(scale[i]) || math.IsInf(scale[i], 0) {
			return nil, fmt.Errorf("fit matrix diagonal %d does not produce a positive finite scale", i)
		}
	}
	scaledMatrix := mat.NewSymDense(n, nil)
	scaledYValues := make([]float64, n)
	for i := range scale {
		for j := i; j < n; j++ {
			scaledMatrix.SetSym(i, j, matrix.At(i, j)/(scale[i]*scale[j]))
		}
		scaledYValues[i] = yValues[i] / scale[i]
	}
	var coefficients mat.VecDense
	if err := coefficients.SolveVec(scaledMatrix, mat.NewVecDense(n, scaledYValues)); err != nil {
		return nil, err
	}
	values := make([]float64, n)
	for i := range values {
		values[i] = coefficients.AtVec(i) / scale[i]
		if math.IsNaN(values[i]) || math.IsInf(values[i], 0) {
			return nil, errors.New("fitted coefficient is not finite")
		}
		if values[i] < 0 {
			values[i] = 0
		}
	}
	return values, nil
}
