from langchain_ollama import ChatOllama

llm_ollama_mistral = ChatOllama(
    model="mistral",
    temperature=0,
    # other params...
)

if __name__ == "__main__":
    messages = [
        (
            "system",
            "You are a helpful assistant that translates English to French. Translate the user sentence.",
        ),
        ("human", "I love programming."),
    ]
    ai_msg = llm_ollama_mistral.invoke(messages)
    print(ai_msg)