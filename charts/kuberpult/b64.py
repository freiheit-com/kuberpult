import base64
import fileinput
import sys
from base64 import urlsafe_b64encode


def main():
    data = sys.stdin.read()
    print(urlsafe_b64encode(bytes(data,encoding='utf8')))

if __name__ == "__main__":
    main()

