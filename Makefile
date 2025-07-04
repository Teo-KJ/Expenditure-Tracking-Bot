APP_NAME=text-classifier-for-expenditure-tracking-bot
VENV=.venv
PYTHON=$(VENV)/bin/python
PIP=$(VENV)/bin/pip
UVICORN=$(VENV)/bin/uvicorn

# === Install dependencies into venv ===
install:
	python3 -m venv $(VENV)
	$(PIP) install --upgrade pip
	$(PIP) install -r requirements.txt

# === Run the FastAPI service ===
run:
	$(UVICORN) text_classifier.app:app --reload --host 0.0.0.0 --port 8000

# === Freeze current packages ===
freeze:
	$(PIP) freeze > requirements.txt

# === Clean up the virtual environment ===
clean:
	rm -rf $(VENV)

# === Run service + auto-install if needed ===
start: install run
