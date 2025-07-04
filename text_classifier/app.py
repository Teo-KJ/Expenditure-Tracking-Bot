from fastapi import FastAPI, Request
from pydantic import BaseModel
from transformers import pipeline

app = FastAPI()

# Load the classifier
classifier = pipeline("text-classification", model="mrm8488/bert-mini-finetuned-expense-category")

class InputData(BaseModel):
    text: str

@app.post("/predict")
def predict(data: InputData):
    result = classifier(data.text)
    return {"label": result[0]['label'], "score": result[0]['score']}
