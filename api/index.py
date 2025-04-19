# autopep8: off
import sys
import os
sys.path.append(os.path.dirname(__file__))
from app import create_app
# autopep8: on

app = create_app()

if __name__ == "__main__":
    app.run(port=5328, debug=False)
