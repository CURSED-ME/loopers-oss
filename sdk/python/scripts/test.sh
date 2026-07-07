#!/bin/bash
set -e
python -m pip install -e .[dev]
pytest
