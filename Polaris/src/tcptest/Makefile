TESTNUM := 10
CXXFLAGS := -O2 -std=c++14 -m64 -DTESTNUMBER=$(TESTNUM)

all: testclient testserver

testclient: testclient.cpp
	$(CXX) $(CXXFLAGS) -o testclient $<

testserver: testserver.cpp
	$(CXX) $(CXXFLAGS) -o testserver $<

clean:
	rm testclient
	rm testserver