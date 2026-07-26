# Technical Psychometrics: 3PL & DIF Engine

This document details the mathematical algorithms and statistical engines behind AEGIS.

## Item Response Theory (IRT) 3-Parameter Logistic Model

For each question (item), the probability $P_i(\theta)$ that a candidate with latent ability $\theta$ answers correctly is modeled as:

$$P_i(\theta) = c_i + \frac{1 - c_i}{1 + e^{-a_i(\theta - b_i)}}$$

Where:
* $a_i$: **Discrimination parameter** (slope of Item Characteristic Curve at inflection point)
* $b_i$: **Difficulty parameter** (location on theta scale, in logits)
* $c_i$: **Pseudo-guessing parameter** (probability of correct guess by low-ability candidate)

### 1. Ability Estimation
* **Expected A Posteriori (EAP)**: Evaluates the posterior distribution using Gauss-Hermite quadrature.
* **Maximum Likelihood Estimation (MLE)**: Solves for the root of log-likelihood derivative using Newton-Raphson iterations.

### 2. Differential Item Functioning (DIF)
To ensure fairness, we perform **Mantel-Haenszel DIF detection** across reference and focal groups matching on raw scores:

$$\alpha_{MH} = \frac{\sum_j (A_j D_j / N_j)}{\sum_j (B_j C_j / N_j)}$$

Where:
* $A_j, B_j$: Reference group counts (correct, incorrect) at total score level $j$.
* $C_j, D_j$: Focal group counts (correct, incorrect) at total score level $j$.
* $N_j$: Total count at score level $j$.

Items are classified into Educational Testing Service (ETS) categories (A, B, or C). Category C items are flagged for automatic review or retirement.
