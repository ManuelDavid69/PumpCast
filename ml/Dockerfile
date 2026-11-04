FROM python:3.12-slim AS base

# Sortie Python non bufferisée et pas de fichiers .pyc
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

# Les dépendances d'abord pour profiter du cache Docker
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Puis le code
COPY . .

# Utilisateur non root
RUN useradd --create-home --uid 1000 pumpcast
USER pumpcast

CMD ["python", "-m", "pumpcast_ml.sync_external"]

# Image pour le notebook d'exploration : la même, plus Jupyter et matplotlib.
# Séparée pour ne pas alourdir l'image utilisée par les tâches planifiées.
FROM base AS notebook
USER root
COPY requirements-notebook.txt .
RUN pip install --no-cache-dir -r requirements-notebook.txt
USER pumpcast
EXPOSE 8888
CMD ["jupyter", "lab", "--ip=0.0.0.0", "--port=8888", "--no-browser", "--notebook-dir=/app/notebooks"]
